package templatesync

import (
	"path/filepath"
	"testing"

	"github.com/bilalbayram/opensnitch-web/internal/db"
	"github.com/bilalbayram/opensnitch-web/internal/nodemanager"
	ruleutil "github.com/bilalbayram/opensnitch-web/internal/rules"
	pb "github.com/bilalbayram/opensnitch-web/proto"
)

func newTemplateSyncTestEnv(t *testing.T) (*Service, *db.Database, *nodemanager.Manager) {
	t.Helper()

	database, err := db.New(filepath.Join(t.TempDir(), "opensnitch-web.db"))
	if err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	nodes := nodemanager.NewManager()
	return New(database, nodes), database, nodes
}

func seedTemplateSyncNode(t *testing.T, database *db.Database, addr string) {
	t.Helper()

	if err := database.UpsertNode(&db.Node{
		Addr:     addr,
		Hostname: addr,
		Status:   "offline",
	}); err != nil {
		t.Fatalf("seed node: %v", err)
	}
}

func createTemplateRule(t *testing.T, database *db.Database, templateID int64, name, operand, data string, position int) *db.TemplateRule {
	t.Helper()

	operatorJSON, err := ruleutil.CanonicalOperatorJSON(&pb.Operator{
		Type:    "simple",
		Operand: operand,
		Data:    data,
	})
	if err != nil {
		t.Fatalf("canonical operator json: %v", err)
	}

	rule, err := database.CreateTemplateRule(&db.TemplateRule{
		TemplateID:      templateID,
		Position:        position,
		Name:            name,
		Enabled:         true,
		Action:          "allow",
		Duration:        "always",
		OperatorType:    "simple",
		OperatorOperand: operand,
		OperatorData:    data,
		OperatorJSON:    operatorJSON,
	})
	if err != nil {
		t.Fatalf("create template rule: %v", err)
	}

	return rule
}

func TestResolveManagedRulesPrefersDirectNodeAttachmentOnPriorityTie(t *testing.T) {
	service, database, _ := newTemplateSyncTestEnv(t)
	seedTemplateSyncNode(t, database, "node-a")

	if _, err := database.ReplaceNodeTags("node-a", []string{"server"}); err != nil {
		t.Fatalf("replace node tags: %v", err)
	}

	tagTemplate, err := database.CreateRuleTemplate(&db.RuleTemplate{Name: "Tag Template"})
	if err != nil {
		t.Fatalf("create tag template: %v", err)
	}
	createTemplateRule(t, database, tagTemplate.ID, "Tag Rule", "process.path", "/usr/bin/curl", 1)
	if _, err := database.CreateTemplateAttachment(&db.TemplateAttachment{
		TemplateID: tagTemplate.ID,
		TargetType: "tag",
		TargetRef:  "server",
		Priority:   10,
	}); err != nil {
		t.Fatalf("create tag attachment: %v", err)
	}

	nodeTemplate, err := database.CreateRuleTemplate(&db.RuleTemplate{Name: "Node Template"})
	if err != nil {
		t.Fatalf("create node template: %v", err)
	}
	createTemplateRule(t, database, nodeTemplate.ID, "Node Rule", "process.path", "/usr/bin/curl", 1)
	if _, err := database.CreateTemplateAttachment(&db.TemplateAttachment{
		TemplateID: nodeTemplate.ID,
		TargetType: "node",
		TargetRef:  "node-a",
		Priority:   10,
	}); err != nil {
		t.Fatalf("create node attachment: %v", err)
	}

	desired, _, err := service.ResolveManagedRules("node-a")
	if err != nil {
		t.Fatalf("resolve managed rules: %v", err)
	}

	if len(desired) != 1 {
		t.Fatalf("expected one resolved rule, got %d", len(desired))
	}
	if desired[0].DisplayName != "Node Rule" {
		t.Fatalf("expected direct node template rule to win, got %+v", desired[0])
	}
}

func TestReconcileNodeQueuesManagedRulesAndPersistsSnapshot(t *testing.T) {
	service, database, nodes := newTemplateSyncTestEnv(t)
	seedTemplateSyncNode(t, database, "node-a")
	nodes.AddNode("node-a", &pb.ClientConfig{Name: "node-a"})

	template, err := database.CreateRuleTemplate(&db.RuleTemplate{Name: "Server Baseline"})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	rule := createTemplateRule(t, database, template.ID, "Allow Curl", "process.path", "/usr/bin/curl", 1)
	if _, err := database.CreateTemplateAttachment(&db.TemplateAttachment{
		TemplateID: template.ID,
		TargetType: "node",
		TargetRef:  "node-a",
		Priority:   100,
	}); err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	if err := service.ReconcileNode("node-a"); err != nil {
		t.Fatalf("reconcile node: %v", err)
	}

	node := nodes.GetNode("node-a")
	if node == nil {
		t.Fatal("expected online node")
	}

	select {
	case notif := <-node.NotifyChan:
		if notif.GetType() != pb.Action_CHANGE_RULE {
			t.Fatalf("expected CHANGE_RULE notification, got %v", notif.GetType())
		}
		if len(notif.GetRules()) != 1 || notif.GetRules()[0].GetName() != ruleutil.ManagedRuleName(template.ID, rule.ID) {
			t.Fatalf("unexpected notification payload: %+v", notif.GetRules())
		}
	default:
		t.Fatal("expected queued notification")
	}

	managedRules, err := database.GetManagedRules("node-a")
	if err != nil {
		t.Fatalf("get managed rules: %v", err)
	}
	if len(managedRules) != 1 || managedRules[0].DisplayName != "Allow Curl" {
		t.Fatalf("unexpected managed rules snapshot: %+v", managedRules)
	}

	state, err := database.GetNodeTemplateSync("node-a")
	if err != nil {
		t.Fatalf("get node sync state: %v", err)
	}
	if state.Pending || state.Error != "" {
		t.Fatalf("expected clean sync state, got %+v", state)
	}
}

func TestReconcileNodeMarksOfflineNodesPending(t *testing.T) {
	service, database, _ := newTemplateSyncTestEnv(t)
	seedTemplateSyncNode(t, database, "node-a")

	template, err := database.CreateRuleTemplate(&db.RuleTemplate{Name: "Offline Template"})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	createTemplateRule(t, database, template.ID, "Allow Curl", "process.path", "/usr/bin/curl", 1)
	if _, err := database.CreateTemplateAttachment(&db.TemplateAttachment{
		TemplateID: template.ID,
		TargetType: "node",
		TargetRef:  "node-a",
		Priority:   100,
	}); err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	if err := service.ReconcileNode("node-a"); err != nil {
		t.Fatalf("reconcile offline node: %v", err)
	}

	state, err := database.GetNodeTemplateSync("node-a")
	if err != nil {
		t.Fatalf("get node sync state: %v", err)
	}
	if !state.Pending || state.Error != "node offline" {
		t.Fatalf("expected pending offline state, got %+v", state)
	}
}
