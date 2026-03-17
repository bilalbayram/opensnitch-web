package grpcserver

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/peer"

	"github.com/evilsocket/opensnitch-web/internal/db"
	"github.com/evilsocket/opensnitch-web/internal/nodemanager"
	"github.com/evilsocket/opensnitch-web/internal/prompter"
	ruleutil "github.com/evilsocket/opensnitch-web/internal/rules"
	"github.com/evilsocket/opensnitch-web/internal/templatesync"
	"github.com/evilsocket/opensnitch-web/internal/ws"
	pb "github.com/evilsocket/opensnitch-web/proto"
)

const testProcessPath = "/opt/opensnitch/test-app"

type serviceTestAddr string

func (a serviceTestAddr) Network() string { return "test" }
func (a serviceTestAddr) String() string  { return string(a) }

type serviceTestEnv struct {
	service  *UIService
	database *db.Database
	prompter *prompter.Prompter
}

func newServiceTestEnv(t *testing.T, promptTimeout int) *serviceTestEnv {
	t.Helper()

	database, err := db.New(filepath.Join(t.TempDir(), "opensnitch-web.db"))
	if err != nil {
		t.Fatalf("create database: %v", err)
	}

	env := &serviceTestEnv{
		database: database,
		prompter: prompter.New(promptTimeout),
	}
	nodes := nodemanager.NewManager()
	env.service = NewUIService(nodes, database, ws.NewHub(), env.prompter, templatesync.New(database, nodes))

	t.Cleanup(func() {
		_ = database.Close()
	})

	return env
}

func (env *serviceTestEnv) seedNode(t *testing.T, addr string) {
	t.Helper()

	if err := env.database.UpsertNode(&db.Node{
		Addr:          addr,
		Hostname:      "test-node",
		Status:        "online",
		LastConn:      "2026-03-16 00:00:00",
		DaemonVersion: "1.0.0",
	}); err != nil {
		t.Fatalf("seed node: %v", err)
	}

	if err := env.database.SetNodeMode(addr, "ask"); err != nil {
		t.Fatalf("set node mode: %v", err)
	}
}

func askRuleContext(addr string) context.Context {
	return peer.NewContext(context.Background(), &peer.Peer{Addr: serviceTestAddr(addr)})
}

func testConnection() *pb.Connection {
	return &pb.Connection{
		ProcessPath: testProcessPath,
		DstHost:     "example.com",
		DstIp:       "93.184.216.34",
		DstPort:     443,
		Protocol:    "tcp",
	}
}

func testSeenFlowKey(node string) db.SeenFlowKey {
	return db.SeenFlowKey{
		Node:               node,
		Process:            testProcessPath,
		Protocol:           "tcp",
		DstPort:            443,
		DestinationOperand: "dest.host",
		Destination:        "example.com",
	}
}

func replyRule(action string) *pb.Rule {
	return replyRuleWithDuration(action, "always")
}

func replyRuleWithDuration(action, duration string) *pb.Rule {
	return &pb.Rule{
		Name:     "web-rule",
		Action:   action,
		Duration: duration,
		Enabled:  true,
		Operator: &pb.Operator{
			Type:    "simple",
			Operand: "process.path",
			Data:    testProcessPath,
		},
	}
}

func TestAskRulePromptsForNewFlowAndPersistsExplicitDecision(t *testing.T) {
	env := newServiceTestEnv(t, 1)
	nodeAddr := "node-a"
	env.seedNode(t, nodeAddr)

	var promptCount atomic.Int32
	env.prompter.OnNewPrompt = func(prompt *prompter.PendingPrompt) {
		promptCount.Add(1)
		if err := env.prompter.Reply(prompt.ID, replyRule("allow")); err != nil {
			t.Errorf("reply prompt: %v", err)
		}
	}

	rule, err := env.service.AskRule(askRuleContext(nodeAddr), testConnection())
	if err != nil {
		t.Fatalf("ask rule: %v", err)
	}
	if rule.GetAction() != "allow" {
		t.Fatalf("expected allow action, got %q", rule.GetAction())
	}
	if promptCount.Load() != 1 {
		t.Fatalf("expected one prompt, got %d", promptCount.Load())
	}

	flow, err := env.database.GetSeenFlow(testSeenFlowKey(nodeAddr))
	if err != nil {
		t.Fatalf("get seen flow: %v", err)
	}
	if flow == nil {
		t.Fatal("expected seen flow to be persisted")
	}
	if flow.Action != "allow" || flow.Count != 1 {
		t.Fatalf("unexpected persisted seen flow: %+v", flow)
	}
}

func TestAskRuleReusesRememberedAllowWithoutPrompting(t *testing.T) {
	env := newServiceTestEnv(t, 1)
	nodeAddr := "node-a"
	env.seedNode(t, nodeAddr)

	var promptCount atomic.Int32
	env.prompter.OnNewPrompt = func(prompt *prompter.PendingPrompt) {
		promptCount.Add(1)
		if err := env.prompter.Reply(prompt.ID, replyRule("allow")); err != nil {
			t.Errorf("reply prompt: %v", err)
		}
	}

	if _, err := env.service.AskRule(askRuleContext(nodeAddr), testConnection()); err != nil {
		t.Fatalf("seed seen flow via prompt: %v", err)
	}

	env.prompter.OnNewPrompt = func(prompt *prompter.PendingPrompt) {
		promptCount.Add(1)
		if err := env.prompter.Reply(prompt.ID, replyRule("allow")); err != nil {
			t.Errorf("reply unexpected prompt: %v", err)
		}
	}

	rule, err := env.service.AskRule(askRuleContext(nodeAddr), testConnection())
	if err != nil {
		t.Fatalf("reuse remembered flow: %v", err)
	}
	if rule.GetAction() != "allow" {
		t.Fatalf("expected remembered allow, got %q", rule.GetAction())
	}
	if rule.GetOperator() == nil || rule.GetOperator().GetType() != "list" {
		t.Fatalf("expected compound once rule for remembered flow, got %+v", rule.GetOperator())
	}
	if promptCount.Load() != 1 {
		t.Fatalf("expected no second prompt, got %d prompts", promptCount.Load())
	}

	flow, err := env.database.GetSeenFlow(testSeenFlowKey(nodeAddr))
	if err != nil {
		t.Fatalf("get refreshed seen flow: %v", err)
	}
	if flow == nil || flow.Count != 2 {
		t.Fatalf("expected remembered flow count to refresh, got %+v", flow)
	}
}

func TestAskRuleReusesRememberedDecisionsForDenyAndReject(t *testing.T) {
	for _, action := range []string{"deny", "reject"} {
		t.Run(action, func(t *testing.T) {
			env := newServiceTestEnv(t, 1)
			nodeAddr := "node-a"
			env.seedNode(t, nodeAddr)

			var promptCount atomic.Int32
			env.prompter.OnNewPrompt = func(prompt *prompter.PendingPrompt) {
				promptCount.Add(1)
				if err := env.prompter.Reply(prompt.ID, replyRule(action)); err != nil {
					t.Errorf("reply prompt: %v", err)
				}
			}

			if _, err := env.service.AskRule(askRuleContext(nodeAddr), testConnection()); err != nil {
				t.Fatalf("seed seen flow via prompt: %v", err)
			}

			env.prompter.OnNewPrompt = func(prompt *prompter.PendingPrompt) {
				promptCount.Add(1)
				if err := env.prompter.Reply(prompt.ID, replyRule(action)); err != nil {
					t.Errorf("reply unexpected prompt: %v", err)
				}
			}

			rule, err := env.service.AskRule(askRuleContext(nodeAddr), testConnection())
			if err != nil {
				t.Fatalf("reuse remembered flow: %v", err)
			}
			if rule.GetAction() != action {
				t.Fatalf("expected remembered %s action, got %q", action, rule.GetAction())
			}
			if promptCount.Load() != 1 {
				t.Fatalf("expected no second prompt, got %d prompts", promptCount.Load())
			}
		})
	}
}

func TestSubscribeReplacesNodeRulesAndReconcilesManagedTemplates(t *testing.T) {
	env := newServiceTestEnv(t, 1)
	nodeAddr := "node-a"

	template, err := env.database.CreateRuleTemplate(&db.RuleTemplate{Name: "Reconnect Template"})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	templateRule, err := env.database.CreateTemplateRule(&db.TemplateRule{
		TemplateID:      template.ID,
		Position:        1,
		Name:            "Managed Curl",
		Enabled:         true,
		Action:          "allow",
		Duration:        "always",
		OperatorType:    "simple",
		OperatorOperand: "process.path",
		OperatorData:    testProcessPath,
	})
	if err != nil {
		t.Fatalf("create template rule: %v", err)
	}
	if _, err := env.database.CreateTemplateAttachment(&db.TemplateAttachment{
		TemplateID: template.ID,
		TargetType: "node",
		TargetRef:  nodeAddr,
		Priority:   100,
	}); err != nil {
		t.Fatalf("create template attachment: %v", err)
	}

	if err := env.database.UpsertRule(&db.DBRule{
		Node:            nodeAddr,
		Name:            "stale-rule",
		DisplayName:     "stale-rule",
		Enabled:         true,
		Action:          "allow",
		Duration:        "always",
		OperatorType:    "simple",
		OperatorOperand: "process.path",
		OperatorData:    "/usr/bin/stale",
	}); err != nil {
		t.Fatalf("seed stale rule: %v", err)
	}

	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: serviceTestAddr(nodeAddr)})
	manualRule := &pb.Rule{
		Name:     "manual-live",
		Enabled:  true,
		Action:   "allow",
		Duration: "always",
		Operator: &pb.Operator{
			Type:    "simple",
			Operand: "process.path",
			Data:    "/usr/bin/manual",
		},
	}

	if _, err := env.service.Subscribe(ctx, &pb.ClientConfig{
		Name:    "test-node",
		Version: "1.0.0",
		Rules:   []*pb.Rule{manualRule},
	}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	rules, err := env.database.GetRules(nodeAddr)
	if err != nil {
		t.Fatalf("get rules after subscribe: %v", err)
	}

	names := make([]string, 0, len(rules))
	for _, rule := range rules {
		names = append(names, rule.Name)
	}
	if containsString(names, "stale-rule") {
		t.Fatalf("expected stale rule to be removed, got %+v", names)
	}
	managedName := ruleutil.ManagedRuleName(template.ID, templateRule.ID)
	if !containsString(names, "manual-live") || !containsString(names, managedName) {
		t.Fatalf("expected manual and managed rules after subscribe, got %+v", names)
	}

	node := env.service.nodes.GetNode(nodeAddr)
	if node == nil {
		t.Fatal("expected node to be connected after subscribe")
	}

	select {
	case notif := <-node.NotifyChan:
		if notif.GetType() != pb.Action_CHANGE_RULE {
			t.Fatalf("expected CHANGE_RULE notification, got %v", notif.GetType())
		}
		if len(notif.GetRules()) != 1 || notif.GetRules()[0].GetName() != managedName {
			t.Fatalf("unexpected reconcile notification: %+v", notif.GetRules())
		}
	default:
		t.Fatal("expected template reconcile notification after subscribe")
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestAskRuleTimeoutDoesNotPersistSeenFlow(t *testing.T) {
	env := newServiceTestEnv(t, 0)
	nodeAddr := "node-a"
	env.seedNode(t, nodeAddr)

	rule, err := env.service.AskRule(askRuleContext(nodeAddr), testConnection())
	if err != nil {
		t.Fatalf("ask rule timeout: %v", err)
	}
	if rule.GetAction() != "deny" {
		t.Fatalf("expected timeout deny rule, got %q", rule.GetAction())
	}

	flow, err := env.database.GetSeenFlow(testSeenFlowKey(nodeAddr))
	if err != nil {
		t.Fatalf("get seen flow after timeout: %v", err)
	}
	if flow != nil {
		t.Fatalf("expected timeout to skip seen flow persistence, got %+v", flow)
	}
}

func TestAskRuleTemporaryPromptDecisionExpires(t *testing.T) {
	env := newServiceTestEnv(t, 1)
	nodeAddr := "node-a"
	env.seedNode(t, nodeAddr)

	var promptCount atomic.Int32
	env.prompter.OnNewPrompt = func(prompt *prompter.PendingPrompt) {
		promptCount.Add(1)
		if err := env.prompter.Reply(prompt.ID, replyRuleWithDuration("allow", "5m")); err != nil {
			t.Errorf("reply prompt: %v", err)
		}
	}

	if _, err := env.service.AskRule(askRuleContext(nodeAddr), testConnection()); err != nil {
		t.Fatalf("seed temporary seen flow: %v", err)
	}

	flow, err := env.database.GetSeenFlow(testSeenFlowKey(nodeAddr))
	if err != nil {
		t.Fatalf("get temporary seen flow: %v", err)
	}
	if flow == nil {
		t.Fatal("expected temporary seen flow to be persisted")
	}
	if flow.ExpiresAt == "" {
		t.Fatalf("expected temporary seen flow to carry expiry metadata, got %+v", flow)
	}

	expiredAt := time.Date(2026, 3, 16, 10, 0, 0, 0, time.Local)
	if err := env.database.UpsertSeenFlow(testSeenFlowKey(nodeAddr), "allow", "web-rule", expiredAt.Add(-10*time.Minute), expiredAt); err != nil {
		t.Fatalf("replace temporary seen flow with expired copy: %v", err)
	}

	env.prompter.OnNewPrompt = func(prompt *prompter.PendingPrompt) {
		promptCount.Add(1)
		if err := env.prompter.Reply(prompt.ID, replyRule("deny")); err != nil {
			t.Errorf("reply expired prompt: %v", err)
		}
	}

	rule, err := env.service.AskRule(askRuleContext(nodeAddr), testConnection())
	if err != nil {
		t.Fatalf("ask rule after temporary expiry: %v", err)
	}
	if rule.GetAction() != "deny" {
		t.Fatalf("expected expired temporary decision to prompt again, got %q", rule.GetAction())
	}
	if promptCount.Load() != 2 {
		t.Fatalf("expected a second prompt after expiry, got %d prompts", promptCount.Load())
	}
}

func TestAskRuleDoesNotPersistUntilRestartDecision(t *testing.T) {
	env := newServiceTestEnv(t, 1)
	nodeAddr := "node-a"
	env.seedNode(t, nodeAddr)

	env.prompter.OnNewPrompt = func(prompt *prompter.PendingPrompt) {
		if err := env.prompter.Reply(prompt.ID, replyRuleWithDuration("allow", "until restart")); err != nil {
			t.Errorf("reply prompt: %v", err)
		}
	}

	if _, err := env.service.AskRule(askRuleContext(nodeAddr), testConnection()); err != nil {
		t.Fatalf("ask rule: %v", err)
	}

	flow, err := env.database.GetSeenFlow(testSeenFlowKey(nodeAddr))
	if err != nil {
		t.Fatalf("get seen flow: %v", err)
	}
	if flow != nil {
		t.Fatalf("expected until restart decision to skip seen flow persistence, got %+v", flow)
	}
}
