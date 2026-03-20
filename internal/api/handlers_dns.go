package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bilalbayram/opensnitch-web/internal/db"
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
