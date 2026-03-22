package api

import (
	"database/sql"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/bilalbayram/opensnitch-web/internal/db"
)

func TestHandleGetNodesReturnsEmptyTagLists(t *testing.T) {
	env := newAPITestEnv(t)
	env.seedNode(t, "node-a", false)

	rec := performJSONRequest(t, env.api.handleGetNodes, http.MethodGet, "/api/v1/nodes", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	response := decodeJSON[[]struct {
		Addr string   `json:"addr"`
		Tags []string `json:"tags"`
	}](t, rec)
	if len(response) != 1 {
		t.Fatalf("expected 1 node, got %d", len(response))
	}
	if response[0].Addr != "node-a" {
		t.Fatalf("unexpected node payload: %+v", response[0])
	}
	if response[0].Tags == nil {
		t.Fatalf("expected empty tags slice, got nil")
	}
	if len(response[0].Tags) != 0 {
		t.Fatalf("expected no tags, got %+v", response[0].Tags)
	}
}

func TestHandleGetNodesMarksRouterOnlineFromRecentHeartbeat(t *testing.T) {
	env := newAPITestEnv(t)

	if err := env.database.UpsertNode(&db.Node{
		Addr:          "router-a",
		Hostname:      "router-a",
		DaemonVersion: "conntrack-agent",
		Status:        "online",
		LastConn:      time.Now().Add(-30 * time.Second).Format("2006-01-02 15:04:05"),
		SourceType:    "router",
	}); err != nil {
		t.Fatalf("seed router node: %v", err)
	}

	rec := performJSONRequest(t, env.api.handleGetNodes, http.MethodGet, "/api/v1/nodes", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	response := decodeJSON[[]struct {
		Addr   string `json:"addr"`
		Status string `json:"status"`
		Online bool   `json:"online"`
	}](t, rec)
	if len(response) != 1 {
		t.Fatalf("expected 1 node, got %d", len(response))
	}
	if response[0].Addr != "router-a" {
		t.Fatalf("unexpected node payload: %+v", response[0])
	}
	if !response[0].Online {
		t.Fatalf("expected router node to be online, got %+v", response[0])
	}
	if response[0].Status != "online" {
		t.Fatalf("expected router status online, got %+v", response[0])
	}
}

func TestHandleGetNodesMarksStaleRouterOffline(t *testing.T) {
	env := newAPITestEnv(t)

	if err := env.database.UpsertNode(&db.Node{
		Addr:          "router-a",
		Hostname:      "router-a",
		DaemonVersion: "conntrack-agent",
		Status:        "online",
		LastConn:      time.Now().Add(-2 * time.Minute).Format("2006-01-02 15:04:05"),
		SourceType:    "router",
	}); err != nil {
		t.Fatalf("seed router node: %v", err)
	}

	rec := performJSONRequest(t, env.api.handleGetNodes, http.MethodGet, "/api/v1/nodes", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	response := decodeJSON[[]struct {
		Status string `json:"status"`
		Online bool   `json:"online"`
	}](t, rec)
	if len(response) != 1 {
		t.Fatalf("expected 1 node, got %d", len(response))
	}
	if response[0].Online {
		t.Fatalf("expected stale router node to be offline, got %+v", response[0])
	}
	if response[0].Status != "offline" {
		t.Fatalf("expected stale router status offline, got %+v", response[0])
	}
}

func TestHandleDeleteNodeRemovesStoredNodeData(t *testing.T) {
	env := newAPITestEnv(t)
	env.seedNode(t, "node-a", false)

	template, err := env.database.CreateRuleTemplate(&db.RuleTemplate{Name: "Template", Description: "test"})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	if _, err := env.database.CreateTemplateAttachment(&db.TemplateAttachment{
		TemplateID: template.ID,
		TargetType: "node",
		TargetRef:  "node-a",
		Priority:   1,
	}); err != nil {
		t.Fatalf("create template attachment: %v", err)
	}
	if err := env.database.UpsertRule(&db.DBRule{
		Time:            "2026-03-23 10:00:00",
		Node:            "node-a",
		Name:            "allow-example",
		DisplayName:     "allow-example",
		SourceKind:      db.RuleSourceManual,
		Enabled:         true,
		Action:          "allow",
		Duration:        "always",
		OperatorType:    "simple",
		OperatorOperand: "dest.host",
		OperatorData:    "example.com",
		Created:         "2026-03-23 10:00:00",
	}); err != nil {
		t.Fatalf("upsert rule: %v", err)
	}
	if _, err := env.database.ReplaceNodeTags("node-a", []string{"server"}); err != nil {
		t.Fatalf("replace tags: %v", err)
	}
	if _, err := env.database.AddProcessTrust("node-a", "/opt/test-app", db.TrustLevelTrusted); err != nil {
		t.Fatalf("add process trust: %v", err)
	}
	if err := env.database.InsertConnection(&db.Connection{
		Time:     "2026-03-23 10:00:00",
		Node:     "node-a",
		Action:   "allow",
		Protocol: "tcp",
		DstIP:    "93.184.216.34",
		DstHost:  "example.com",
		DstPort:  443,
		Process:  "/usr/bin/curl",
	}); err != nil {
		t.Fatalf("insert connection: %v", err)
	}
	if err := env.database.InsertAlert(&db.DBAlert{
		Time:   "2026-03-23 10:00:00",
		Node:   "node-a",
		Body:   "test alert",
		Status: "open",
	}); err != nil {
		t.Fatalf("insert alert: %v", err)
	}
	if err := env.database.UpsertSeenFlow(db.SeenFlowKey{
		Node:               "node-a",
		Process:            "/usr/bin/curl",
		Protocol:           "tcp",
		DstPort:            443,
		DestinationOperand: "dest.host",
		Destination:        "example.com",
	}, "allow", "allow-example", time.Now(), time.Time{}); err != nil {
		t.Fatalf("upsert seen flow: %v", err)
	}
	if err := env.database.UpsertDNSDomain("node-a", "example.com", "93.184.216.34", "2026-03-23 10:00:00"); err != nil {
		t.Fatalf("upsert dns domain: %v", err)
	}
	for table, what := range map[string]string{
		"hosts": "example.com",
		"procs": "/opt/test-app",
		"addrs": "93.184.216.34",
		"ports": "443",
		"users": "root",
	} {
		if err := env.database.UpsertStat(table, what, "node-a", 3); err != nil {
			t.Fatalf("upsert %s stat: %v", table, err)
		}
	}

	rec := performJSONRequestWithAddr(t, env.api.handleDeleteNode, http.MethodDelete, "/api/v1/nodes/node-a", "node-a", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if _, err := env.database.GetNode("node-a"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected node to be removed, got %v", err)
	}
	if rules, err := env.database.GetRules("node-a"); err != nil || len(rules) != 0 {
		t.Fatalf("expected rules removed, got %v %+v", err, rules)
	}
	if tags, err := env.database.GetNodeTags("node-a"); err != nil || len(tags) != 0 {
		t.Fatalf("expected tags removed, got %v %+v", err, tags)
	}
	if trust := env.database.GetProcessTrustLevel("node-a", "/opt/test-app"); trust != db.TrustLevelDefault {
		t.Fatalf("expected node-specific trust removed, got %q", trust)
	}
	if _, total, err := env.database.GetConnections(&db.ConnectionFilter{Node: "node-a"}); err != nil || total != 0 {
		t.Fatalf("expected connections removed, got total=%d err=%v", total, err)
	}
	if _, total, err := env.database.GetDNSDomains(&db.DNSDomainFilter{Node: "node-a"}); err != nil || total != 0 {
		t.Fatalf("expected dns domains removed, got total=%d err=%v", total, err)
	}
	if _, total, err := env.database.GetSeenFlows(&db.SeenFlowFilter{Node: "node-a"}); err != nil || total != 0 {
		t.Fatalf("expected seen flows removed, got total=%d err=%v", total, err)
	}
	if attachments, err := env.database.GetAllTemplateAttachments(); err != nil || len(attachments) != 0 {
		t.Fatalf("expected template attachments removed, got %v %+v", err, attachments)
	}
	if alerts, total, err := env.database.GetAlerts(50, 0); err != nil || total != 0 || len(alerts) != 0 {
		t.Fatalf("expected alerts removed, got total=%d len=%d err=%v", total, len(alerts), err)
	}
	for _, table := range []string{"hosts", "procs", "addrs", "ports", "users"} {
		entries, err := env.database.GetStats(table, 10)
		if err != nil {
			t.Fatalf("get %s stats: %v", table, err)
		}
		for _, entry := range entries {
			if entry.Node == "node-a" {
				t.Fatalf("expected %s stats removed, got %+v", table, entry)
			}
		}
	}
}

func TestHandleDeleteNodeRejectsOnlineNode(t *testing.T) {
	env := newAPITestEnv(t)
	env.seedNode(t, "node-a", true)

	rec := performJSONRequestWithAddr(t, env.api.handleDeleteNode, http.MethodDelete, "/api/v1/nodes/node-a", "node-a", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDeleteNodeRejectsRouterBackedNode(t *testing.T) {
	env := newAPITestEnv(t)

	if err := env.database.UpsertNode(&db.Node{
		Addr:          "router-node",
		Hostname:      "router-node",
		DaemonVersion: "conntrack-agent",
		Status:        db.NodeStatusOffline,
		LastConn:      "2026-03-23 10:00:00",
		SourceType:    "router",
	}); err != nil {
		t.Fatalf("seed router node: %v", err)
	}

	rec := performJSONRequestWithAddr(t, env.api.handleDeleteNode, http.MethodDelete, "/api/v1/nodes/router-node", "router-node", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDeleteNodeRejectsRouterManagedNode(t *testing.T) {
	env := newAPITestEnv(t)
	env.seedNode(t, "managed-node", false)

	if err := env.database.InsertRouter(&db.Router{
		Name:           "router-a",
		Addr:           "192.168.1.1",
		SSHPort:        22,
		SSHUser:        "root",
		APIKey:         "api-key",
		LANSubnet:      "192.168.1.0/24",
		DaemonMode:     db.RouterDaemonModeRouterDaemon,
		LinkedNodeAddr: "managed-node",
		Status:         "active",
	}); err != nil {
		t.Fatalf("seed router: %v", err)
	}

	rec := performJSONRequestWithAddr(t, env.api.handleDeleteNode, http.MethodDelete, "/api/v1/nodes/managed-node", "managed-node", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}
