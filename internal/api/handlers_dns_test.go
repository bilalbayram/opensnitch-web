package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/bilalbayram/opensnitch-web/internal/dnspolicy"
	"github.com/bilalbayram/opensnitch-web/internal/nodemanager"
	pb "github.com/bilalbayram/opensnitch-web/proto"
)

func TestHandleSetDNSPolicyReplacesExistingDNSRestrictions(t *testing.T) {
	env := newAPITestEnv(t)
	env.seedNode(t, "node-a", true)

	env.insertRule(t, "node-a", &pb.Rule{
		Name:        "dns-allow-8-8-8-8",
		Description: "legacy allow",
		Enabled:     true,
		Precedence:  true,
		Action:      "allow",
		Duration:    "always",
		Operator: &pb.Operator{
			Type: "list",
			List: []*pb.Operator{
				{Type: "simple", Operand: "dest.port", Data: "53"},
				{Type: "simple", Operand: "dest.ip", Data: "8.8.8.8"},
			},
		},
	})
	env.insertRule(t, "node-a", &pb.Rule{
		Name:        "dns-deny-catchall",
		Description: "legacy deny",
		Enabled:     true,
		Action:      "deny",
		Duration:    "always",
		Operator: &pb.Operator{
			Type:    "simple",
			Operand: "dest.port",
			Data:    "53",
		},
	})
	env.insertRule(t, "node-a", &pb.Rule{
		Name:        "dnspol-deny-dot",
		Description: "old policy",
		Enabled:     true,
		Action:      "deny",
		Duration:    "always",
		Operator: &pb.Operator{
			Type:    "simple",
			Operand: "dest.port",
			Data:    "853",
		},
	})

	rec := performJSONRequest(t, env.api.handleSetDNSPolicy, http.MethodPost, "/api/v1/dns/policy", dnsPolicyRequest{
		Node:             "node-a",
		Enabled:          true,
		AllowedResolvers: []string{"1.1.1.1"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	node := env.nodes.GetNode("node-a")
	if node == nil {
		t.Fatal("expected node to be online")
	}

	deleteNotif := expectNotification(t, node)
	if deleteNotif.GetType() != pb.Action_DELETE_RULE {
		t.Fatalf("expected delete notification first, got %+v", deleteNotif)
	}
	deleteNames := notificationRuleNames(deleteNotif)
	for _, name := range []string{"dns-allow-8-8-8-8", "dns-deny-catchall", "dnspol-deny-dot"} {
		if _, ok := deleteNames[name]; !ok {
			t.Fatalf("expected delete notification to include %q, got %+v", name, deleteNotif.GetRules())
		}
	}

	changeNotif := expectNotification(t, node)
	if changeNotif.GetType() != pb.Action_CHANGE_RULE {
		t.Fatalf("expected change notification second, got %+v", changeNotif)
	}
	changeNames := notificationRuleNames(changeNotif)
	for _, name := range []string{"dnspol-allow-dns-1-1-1-1", "dnspol-deny-dns-catchall"} {
		if _, ok := changeNames[name]; !ok {
			t.Fatalf("expected change notification to include %q, got %+v", name, changeNotif.GetRules())
		}
	}
	if _, ok := changeNames["dnspol-deny-dot"]; ok {
		t.Fatalf("did not expect old DoT rule to remain in change set: %+v", changeNotif.GetRules())
	}
	expectNoNotification(t, node)

	for _, name := range []string{"dns-allow-8-8-8-8", "dns-deny-catchall", "dnspol-deny-dot"} {
		if _, err := env.database.GetRule("node-a", name); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("expected %q to be deleted from db, got err=%v", name, err)
		}
	}
	if _, err := env.database.GetRule("node-a", "dnspol-allow-dns-1-1-1-1"); err != nil {
		t.Fatalf("expected new allow rule in db: %v", err)
	}

	cfgJSON, err := env.database.GetWebConfig(dnspolicy.ConfigKey("node-a"))
	if err != nil {
		t.Fatalf("expected stored policy config: %v", err)
	}
	var policy dnspolicy.DNSPolicy
	if err := json.Unmarshal([]byte(cfgJSON), &policy); err != nil {
		t.Fatalf("decode policy config: %v", err)
	}
	if !policy.Enabled || len(policy.AllowedResolvers) != 1 || policy.AllowedResolvers[0] != "1.1.1.1" {
		t.Fatalf("unexpected stored policy: %+v", policy)
	}
}

func TestHandleSetDNSPolicyCreatesNodeScopedDoHHostnameRules(t *testing.T) {
	env := newAPITestEnv(t)
	env.seedNode(t, "node-a", true)

	rec := performJSONRequest(t, env.api.handleSetDNSPolicy, http.MethodPost, "/api/v1/dns/policy", dnsPolicyRequest{
		Node:              "node-a",
		Enabled:           true,
		AllowedResolvers:  []string{"1.1.1.1"},
		BlockDoHHostnames: true,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	node := env.nodes.GetNode("node-a")
	if node == nil {
		t.Fatal("expected node to be online")
	}

	notif := expectNotification(t, node)
	if notif.GetType() != pb.Action_CHANGE_RULE {
		t.Fatalf("expected a change notification, got %+v", notif)
	}

	hostRuleCount := 0
	for _, rule := range notif.GetRules() {
		if rule == nil || !strings.HasPrefix(rule.GetName(), "dnspol-deny-doh-host-") {
			continue
		}
		hostRuleCount++
		if len(rule.GetOperator().GetList()) != 2 {
			t.Fatalf("expected host deny rule to have 2 operators, got %+v", rule.GetOperator())
		}
		if rule.GetOperator().GetList()[0].GetOperand() != "dest.port" || rule.GetOperator().GetList()[0].GetData() != "443" {
			t.Fatalf("expected host deny rule to match port 443, got %+v", rule.GetOperator())
		}
		if rule.GetOperator().GetList()[1].GetOperand() != "dest.host" {
			t.Fatalf("expected host deny rule to use dest.host, got %+v", rule.GetOperator())
		}
	}
	if hostRuleCount != len(dnspolicy.DoHHostnames) {
		t.Fatalf("expected %d host deny rules, got %d", len(dnspolicy.DoHHostnames), hostRuleCount)
	}
	expectNoNotification(t, node)

	lists, err := env.database.GetBlocklists()
	if err != nil {
		t.Fatalf("list blocklists: %v", err)
	}
	for _, list := range lists {
		if list.URL == "builtin://doh-hostnames" {
			t.Fatalf("did not expect a global DoH hostname blocklist entry, found %+v", list)
		}
	}
}

func TestHandleSetDNSPolicyDisableRequiresConnectedNode(t *testing.T) {
	env := newAPITestEnv(t)
	env.seedNode(t, "node-a", false)

	env.insertRule(t, "node-a", &pb.Rule{
		Name:        "dnspol-allow-dns-1-1-1-1",
		Description: "existing policy",
		Enabled:     true,
		Precedence:  true,
		Action:      "allow",
		Duration:    "always",
		Operator: &pb.Operator{
			Type: "list",
			List: []*pb.Operator{
				{Type: "simple", Operand: "dest.port", Data: "53"},
				{Type: "simple", Operand: "dest.ip", Data: "1.1.1.1"},
			},
		},
	})

	cfgJSON, err := json.Marshal(dnspolicy.DNSPolicy{
		Enabled:          true,
		AllowedResolvers: []string{"1.1.1.1"},
	})
	if err != nil {
		t.Fatalf("marshal policy config: %v", err)
	}
	if err := env.database.SetWebConfig(dnspolicy.ConfigKey("node-a"), string(cfgJSON)); err != nil {
		t.Fatalf("seed policy config: %v", err)
	}

	rec := performJSONRequest(t, env.api.handleSetDNSPolicy, http.MethodPost, "/api/v1/dns/policy", dnsPolicyRequest{
		Node:    "node-a",
		Enabled: false,
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for offline node, got %d: %s", rec.Code, rec.Body.String())
	}

	if _, err := env.database.GetRule("node-a", "dnspol-allow-dns-1-1-1-1"); err != nil {
		t.Fatalf("expected policy rule to remain in db after failed disable: %v", err)
	}
	if cfg, err := env.database.GetWebConfig(dnspolicy.ConfigKey("node-a")); err != nil || cfg == "" {
		t.Fatalf("expected policy config to remain after failed disable: cfg=%q err=%v", cfg, err)
	}
}

func expectNotification(t *testing.T, node *nodemanager.NodeState) *pb.Notification {
	t.Helper()

	select {
	case notif := <-node.NotifyChan:
		return notif
	default:
		t.Fatal("expected queued notification")
		return nil
	}
}

func expectNoNotification(t *testing.T, node *nodemanager.NodeState) {
	t.Helper()

	select {
	case notif := <-node.NotifyChan:
		t.Fatalf("unexpected notification: %+v", notif)
	default:
	}
}

func notificationRuleNames(notif *pb.Notification) map[string]struct{} {
	names := make(map[string]struct{}, len(notif.GetRules()))
	for _, rule := range notif.GetRules() {
		if rule == nil {
			continue
		}
		names[rule.GetName()] = struct{}{}
	}
	return names
}
