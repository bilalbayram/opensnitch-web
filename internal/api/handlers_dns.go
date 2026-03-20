package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bilalbayram/opensnitch-web/internal/db"
	"github.com/bilalbayram/opensnitch-web/internal/dnspolicy"
	ruleutil "github.com/bilalbayram/opensnitch-web/internal/rules"
	"github.com/bilalbayram/opensnitch-web/internal/ws"
	pb "github.com/bilalbayram/opensnitch-web/proto"
)

func (a *API) handleGetDNSDomains(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	filter := &db.DNSDomainFilter{
		Node:   q.Get("node"),
		Domain: q.Get("domain"),
		IP:     q.Get("ip"),
		Search: q.Get("search"),
		Limit:  limit,
		Offset: offset,
	}

	domains, total, err := a.db.GetDNSDomains(filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if domains == nil {
		domains = []db.DNSDomain{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  domains,
		"total": total,
	})
}

func (a *API) handleGetDNSServers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	node := q.Get("node")

	summaries, total, err := a.db.GetDNSServerSummary(node, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if summaries == nil {
		summaries = []db.DNSServerSummary{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  summaries,
		"total": total,
	})
}

func (a *API) handlePurgeDNSDomains(w http.ResponseWriter, r *http.Request) {
	if err := a.db.PurgeDNSDomains(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type dnsServerRulesRequest struct {
	Node        string   `json:"node"`
	AllowedIPs  []string `json:"allowed_ips"`
	Description string   `json:"description"`
}

func (a *API) handleCreateDNSServerRules(w http.ResponseWriter, r *http.Request) {
	var req dnsServerRulesRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	req.Node = strings.TrimSpace(req.Node)
	if req.Node == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node is required"})
		return
	}

	// Check if DNS policy is active for this node
	if cfgJSON, err := a.db.GetWebConfig(dnspolicy.ConfigKey(req.Node)); err == nil && cfgJSON != "" {
		var pol dnspolicy.DNSPolicy
		if json.Unmarshal([]byte(cfgJSON), &pol) == nil && pol.Enabled {
			writeJSON(w, http.StatusConflict, map[string]string{
				"error": "DNS policy is active for this node. Use the DNS Policy settings to manage DNS restrictions.",
			})
			return
		}
	}

	ips := normalizeStringList(req.AllowedIPs)
	if len(ips) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one IP is required"})
		return
	}

	if a.nodes.GetNode(req.Node) == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "node not connected"})
		return
	}

	desc := req.Description
	if desc == "" {
		desc = "DNS server restriction rule"
	}

	// Remove stale dns-allow-* rules for this node before creating the new set
	if err := a.db.DeleteDNSAllowRules(req.Node); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	now := time.Now()
	var allProtoRules []*pb.Rule
	var allResponses []*ruleResponse
	var allRuleNames []string

	// Create an ALLOW rule per IP: dest.port=53 AND dest.ip=<ip>
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}

		slug := strings.ReplaceAll(ip, ".", "-")
		slug = strings.ReplaceAll(slug, ":", "-")
		name := "dns-allow-" + slug

		protoRule := &pb.Rule{
			Name:        name,
			Description: desc,
			Enabled:     true,
			Precedence:  true,
			Action:      "allow",
			Duration:    "always",
			Operator: &pb.Operator{
				Type: "list",
				List: []*pb.Operator{
					{Type: "simple", Operand: "dest.port", Data: "53"},
					{Type: "simple", Operand: "dest.ip", Data: ip},
				},
			},
		}

		dbRule, err := ruleutil.ProtoToDBRule(req.Node, now, protoRule)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if err := a.db.UpsertRule(dbRule); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		resp, err := a.buildRuleResponse(dbRule)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		allProtoRules = append(allProtoRules, protoRule)
		allResponses = append(allResponses, resp)
		allRuleNames = append(allRuleNames, name)
	}

	// Create a DENY catch-all for port 53
	denyRule := &pb.Rule{
		Name:        "dns-deny-catchall",
		Description: "Deny DNS to non-allowed servers",
		Enabled:     true,
		Precedence:  false,
		Action:      "deny",
		Duration:    "always",
		Operator: &pb.Operator{
			Type:    "simple",
			Operand: "dest.port",
			Data:    "53",
		},
	}

	dbDeny, err := ruleutil.ProtoToDBRule(req.Node, now, denyRule)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := a.db.UpsertRule(dbDeny); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	denyResp, err := a.buildRuleResponse(dbDeny)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	allProtoRules = append(allProtoRules, denyRule)
	allResponses = append(allResponses, denyResp)
	allRuleNames = append(allRuleNames, "dns-deny-catchall")

	// Push all rules to daemon; rollback DB writes on failure
	if !a.pushRulesToDaemon(req.Node, allProtoRules, pb.Action_CHANGE_RULE) {
		for _, name := range allRuleNames {
			_ = a.db.DeleteRule(req.Node, name)
		}
		writeJSON(w, http.StatusConflict, map[string]string{"error": "node notification queue is full"})
		return
	}

	// Broadcast WS events
	for _, resp := range allResponses {
		a.hub.BroadcastEvent(ws.EventRuleChanged, map[string]any{
			"action": "created",
			"rule":   resp,
		})
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"status": "ok",
		"data":   allResponses,
		"count":  len(allResponses),
	})
}

// ─── DNS Policy Handlers ────────────────────────────────────────────

type dnsPolicyRequest struct {
	Node              string   `json:"node"`
	Enabled           bool     `json:"enabled"`
	AllowedResolvers  []string `json:"allowed_resolvers"`
	BlockDoT          bool     `json:"block_dot"`
	BlockDoHIPs       bool     `json:"block_doh_ips"`
	BlockDoHHostnames bool     `json:"block_doh_hostnames"`
}

func (a *API) handleGetDNSPolicy(w http.ResponseWriter, r *http.Request) {
	node := r.URL.Query().Get("node")

	// If node specified, return single policy
	if node != "" {
		cfgJSON, err := a.db.GetWebConfig(dnspolicy.ConfigKey(node))
		if err != nil {
			// No policy found
			writeJSON(w, http.StatusOK, map[string]any{
				"policy":     nil,
				"rule_count": 0,
			})
			return
		}

		var pol dnspolicy.DNSPolicy
		if err := json.Unmarshal([]byte(cfgJSON), &pol); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "corrupt policy config"})
			return
		}

		ruleNames, _ := a.db.GetDNSPolicyRuleNames(node)
		writeJSON(w, http.StatusOK, map[string]any{
			"policy":     pol,
			"rule_count": len(ruleNames),
		})
		return
	}

	// No node: return all policies
	entries, err := a.db.GetWebConfigByPrefix(dnspolicy.ConfigPrefix)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	policies := make(map[string]dnspolicy.DNSPolicy)
	for nodeAddr, cfgJSON := range entries {
		var pol dnspolicy.DNSPolicy
		if json.Unmarshal([]byte(cfgJSON), &pol) == nil {
			policies[nodeAddr] = pol
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"policies": policies,
	})
}

func (a *API) handleSetDNSPolicy(w http.ResponseWriter, r *http.Request) {
	var req dnsPolicyRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	req.Node = strings.TrimSpace(req.Node)
	if req.Node == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node is required"})
		return
	}

	// ─── Disable flow ───
	if !req.Enabled {
		if a.nodes.GetNode(req.Node) == nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "node not connected"})
			return
		}
		a.disableDNSPolicy(w, req.Node)
		return
	}

	// ─── Enable flow ───
	resolvers := normalizeStringList(req.AllowedResolvers)
	if len(resolvers) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one resolver IP is required"})
		return
	}

	if a.nodes.GetNode(req.Node) == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "node not connected"})
		return
	}

	deleteNames, err := a.db.GetDNSRestrictionRuleNames(req.Node)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	policy := dnspolicy.DNSPolicy{
		Enabled:           true,
		AllowedResolvers:  resolvers,
		BlockDoT:          req.BlockDoT,
		BlockDoHIPs:       req.BlockDoHIPs,
		BlockDoHHostnames: req.BlockDoHHostnames,
		EnabledAt:         time.Now().Format("2006-01-02 15:04:05"),
	}

	protoRules := dnspolicy.BuildRules(policy)

	now := time.Now()
	dbRules := make([]*db.DBRule, 0, len(protoRules))

	for _, pr := range protoRules {
		dbRule, err := ruleutil.ProtoToDBRule(req.Node, now, pr)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		dbRule.SourceKind = "dns-policy"
		dbRules = append(dbRules, dbRule)
	}

	if !a.sendDNSRuleBatch(req.Node, deleteNames, protoRules) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "node notification queue is full"})
		return
	}

	if err := a.deleteRuleSet(req.Node, deleteNames); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	for _, dbRule := range dbRules {
		if err := a.db.UpsertRule(dbRule); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	cfgJSON, _ := json.Marshal(policy)
	if err := a.db.SetWebConfig(dnspolicy.ConfigKey(req.Node), string(cfgJSON)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Broadcast WS events
	a.hub.BroadcastEvent(ws.EventRuleChanged, map[string]any{
		"action": "dns_policy_enabled",
		"node":   req.Node,
		"count":  len(protoRules),
	})

	writeJSON(w, http.StatusCreated, map[string]any{
		"status":     "ok",
		"rule_count": len(protoRules),
		"policy":     policy,
	})
}

func (a *API) disableDNSPolicy(w http.ResponseWriter, node string) {
	ruleNames, err := a.db.GetDNSPolicyRuleNames(node)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if !a.sendDNSRuleBatch(node, ruleNames, nil) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "node notification queue is full"})
		return
	}

	if err := a.deleteRuleSet(node, ruleNames); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := a.db.DeleteWebConfig(dnspolicy.ConfigKey(node)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	a.hub.BroadcastEvent(ws.EventRuleChanged, map[string]any{
		"action": "dns_policy_disabled",
		"node":   node,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func (a *API) handleGetDNSPolicyProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"doh_ips":       dnspolicy.DoHProviderIPs,
		"doh_hostnames": dnspolicy.DoHHostnames,
	})
}

// isDNSPolicyActive checks whether DNS policy is enabled for a given node.
func (a *API) isDNSPolicyActive(node string) bool {
	cfgJSON, err := a.db.GetWebConfig(dnspolicy.ConfigKey(node))
	if err != nil || cfgJSON == "" {
		return false
	}
	var pol dnspolicy.DNSPolicy
	return json.Unmarshal([]byte(cfgJSON), &pol) == nil && pol.Enabled
}

// countEnabledDNSPolicies returns the number of nodes with an active DNS policy.
func (a *API) countEnabledDNSPolicies() int {
	entries, err := a.db.GetWebConfigByPrefix(dnspolicy.ConfigPrefix)
	if err != nil {
		return 0
	}
	count := 0
	for _, cfgJSON := range entries {
		var pol dnspolicy.DNSPolicy
		if json.Unmarshal([]byte(cfgJSON), &pol) == nil && pol.Enabled {
			count++
		}
	}
	return count
}

func (a *API) sendDNSRuleBatch(node string, deleteNames []string, changeRules []*pb.Rule) bool {
	notifications := make([]*pb.Notification, 0, 2)

	deleteRules := make([]*pb.Rule, 0, len(deleteNames))
	for _, name := range deleteNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		deleteRules = append(deleteRules, &pb.Rule{Name: name})
	}
	if len(deleteRules) > 0 {
		notifications = append(notifications, &pb.Notification{
			Id:    a.nodes.NextID(),
			Type:  pb.Action_DELETE_RULE,
			Rules: deleteRules,
		})
	}
	if len(changeRules) > 0 {
		notifications = append(notifications, &pb.Notification{
			Id:    a.nodes.NextID(),
			Type:  pb.Action_CHANGE_RULE,
			Rules: changeRules,
		})
	}

	return a.nodes.SendNotificationBatch(node, notifications)
}

func (a *API) deleteRuleSet(node string, ruleNames []string) error {
	for _, name := range ruleNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if err := a.db.DeleteRule(node, name); err != nil {
			return err
		}
	}
	return nil
}
