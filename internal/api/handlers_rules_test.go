package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc/peer"

	"github.com/evilsocket/opensnitch-web/internal/config"
	"github.com/evilsocket/opensnitch-web/internal/db"
	"github.com/evilsocket/opensnitch-web/internal/grpcserver"
	"github.com/evilsocket/opensnitch-web/internal/nodemanager"
	"github.com/evilsocket/opensnitch-web/internal/prompter"
	ruleutil "github.com/evilsocket/opensnitch-web/internal/rules"
	"github.com/evilsocket/opensnitch-web/internal/ws"
	pb "github.com/evilsocket/opensnitch-web/proto"
)

type apiTestEnv struct {
	api      *API
	database *db.Database
	nodes    *nodemanager.Manager
	hub      *ws.Hub
	prompter *prompter.Prompter
}

func newAPITestEnv(t *testing.T) *apiTestEnv {
	t.Helper()

	database, err := db.New(filepath.Join(t.TempDir(), "opensnitch-web.db"))
	if err != nil {
		t.Fatalf("create database: %v", err)
	}

	env := &apiTestEnv{
		database: database,
		nodes:    nodemanager.NewManager(),
		hub:      ws.NewHub(),
		prompter: prompter.New(1),
	}
	env.api = &API{
		cfg:      config.DefaultConfig(),
		db:       env.database,
		nodes:    env.nodes,
		hub:      env.hub,
		prompter: env.prompter,
	}

	t.Cleanup(func() {
		_ = database.Close()
	})

	return env
}

func (env *apiTestEnv) seedNode(t *testing.T, addr string, online bool) {
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

	if err := env.database.SetNodeMode(addr, "silent_allow"); err != nil {
		t.Fatalf("set node mode: %v", err)
	}

	if online {
		env.nodes.AddNode(addr, &pb.ClientConfig{Name: "test-node"})
	}
}

func (env *apiTestEnv) insertConnection(t *testing.T, conn db.Connection) {
	t.Helper()

	if err := env.database.InsertConnection(&conn); err != nil {
		t.Fatalf("insert connection: %v", err)
	}
}

func (env *apiTestEnv) insertRule(t *testing.T, node string, rule *pb.Rule) {
	t.Helper()

	dbRule, err := ruleutil.ProtoToDBRule(node, time.Date(2026, 3, 16, 15, 0, 0, 0, time.Local), rule)
	if err != nil {
		t.Fatalf("convert rule: %v", err)
	}
	if err := env.database.UpsertRule(dbRule); err != nil {
		t.Fatalf("insert rule: %v", err)
	}
}

func performJSONRequest(t *testing.T, handler http.HandlerFunc, method, target string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, target, &body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func performJSONRequestWithRuleName(t *testing.T, handler http.HandlerFunc, method, target, ruleName string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, target, &body)
	req.Header.Set("Content-Type", "application/json")

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("name", ruleName)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))

	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()

	var response T
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}

func TestHandleGenerateRulesPreviewValidation(t *testing.T) {
	env := newAPITestEnv(t)

	rec := performJSONRequest(t, env.api.handleGenerateRulesPreview, http.MethodPost, "/api/v1/rules/generate/preview", generateRulesRequest{
		Since: "2026-03-16 10:00:00",
		Until: "2026-03-16 12:00:00",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing node, got %d", rec.Code)
	}

	rec = performJSONRequest(t, env.api.handleGenerateRulesPreview, http.MethodPost, "/api/v1/rules/generate/preview", generateRulesRequest{
		Node:  "node-a",
		Since: "2026-03-16 12:00:00",
		Until: "2026-03-16 10:00:00",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid range, got %d", rec.Code)
	}
}

func TestHandleGenerateRulesPreviewSkipsDuplicatesAndSerializesCompoundRules(t *testing.T) {
	env := newAPITestEnv(t)
	env.seedNode(t, "node-a", false)

	env.insertConnection(t, db.Connection{
		Time:        "2026-03-16 10:00:00",
		Node:        "node-a",
		Action:      "allow",
		Protocol:    "tcp",
		SrcIP:       "10.0.0.1",
		SrcPort:     10001,
		DstIP:       "93.184.216.34",
		DstHost:     "example.com",
		DstPort:     443,
		Process:     "/usr/bin/curl",
		ProcessArgs: "--head",
		Rule:        "silent-allow",
	})
	env.insertConnection(t, db.Connection{
		Time:        "2026-03-16 10:15:00",
		Node:        "node-a",
		Action:      "allow",
		Protocol:    "tcp",
		SrcIP:       "10.0.0.2",
		SrcPort:     2200,
		DstIP:       "203.0.113.10",
		DstHost:     "ssh.example.com",
		DstPort:     22,
		Process:     "/usr/bin/bash",
		ProcessArgs: "-lc ssh",
		Rule:        "silent-allow",
	})
	env.insertConnection(t, db.Connection{
		Time:        "2026-03-16 11:00:00",
		Node:        "node-a",
		Action:      "allow",
		Protocol:    "udp",
		SrcIP:       "10.0.0.3",
		SrcPort:     5300,
		DstIP:       "1.1.1.1",
		DstPort:     53,
		Process:     "/usr/bin/dig",
		ProcessArgs: "@1.1.1.1",
		Rule:        "silent-allow",
	})

	curlKey := ruleutil.LearningKey{
		Process:         "/usr/bin/curl",
		DestinationType: "dest.host",
		Destination:     "example.com",
		DstPort:         443,
		Protocol:        "tcp",
	}
	env.insertRule(t, "node-a", ruleutil.BuildGeneratedRule(curlKey))

	rec := performJSONRequest(t, env.api.handleGenerateRulesPreview, http.MethodPost, "/api/v1/rules/generate/preview", generateRulesRequest{
		Node:             "node-a",
		Since:            "2026-03-16 09:00:00",
		Until:            "2026-03-16 12:00:00",
		ExcludeProcesses: []string{"/usr/bin/bash"},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	response := decodeJSON[generateRulesPreviewResponse](t, rec)
	if response.SkippedExisting != 1 {
		t.Fatalf("expected 1 duplicate skip, got %d", response.SkippedExisting)
	}
	if response.SkippedExcluded != 1 {
		t.Fatalf("expected 1 exclusion skip, got %d", response.SkippedExcluded)
	}
	if len(response.Data) != 1 {
		t.Fatalf("expected 1 proposal, got %d", len(response.Data))
	}

	proposal := response.Data[0]
	if proposal.Process != "/usr/bin/dig" {
		t.Fatalf("expected dig proposal, got %+v", proposal)
	}
	if proposal.Rule == nil || !proposal.Rule.IsCompound {
		t.Fatalf("expected compound rule in preview, got %+v", proposal.Rule)
	}
	if proposal.Rule.Operator == nil || proposal.Rule.Operator.GetType() != "list" {
		t.Fatalf("expected compound operator serialization, got %+v", proposal.Rule.Operator)
	}
	if len(proposal.Rule.Operator.GetList()) != 4 {
		t.Fatalf("expected 4 child operators, got %d", len(proposal.Rule.Operator.GetList()))
	}
}

func TestHandleGenerateRulesApplyRequiresOnlineNode(t *testing.T) {
	env := newAPITestEnv(t)
	env.seedNode(t, "node-a", false)

	env.insertConnection(t, db.Connection{
		Time:        "2026-03-16 10:00:00",
		Node:        "node-a",
		Action:      "allow",
		Protocol:    "tcp",
		SrcIP:       "10.0.0.1",
		SrcPort:     10001,
		DstIP:       "93.184.216.34",
		DstHost:     "example.com",
		DstPort:     443,
		Process:     "/usr/bin/curl",
		ProcessArgs: "--head",
		Rule:        "silent-allow",
	})

	key := ruleutil.LearningKey{
		Process:         "/usr/bin/curl",
		DestinationType: "dest.host",
		Destination:     "example.com",
		DstPort:         443,
		Protocol:        "tcp",
	}

	rec := performJSONRequest(t, env.api.handleGenerateRulesApply, http.MethodPost, "/api/v1/rules/generate/apply", applyGeneratedRulesRequest{
		Node:         "node-a",
		Since:        "2026-03-16 09:00:00",
		Until:        "2026-03-16 12:00:00",
		Fingerprints: []string{ruleutil.FingerprintForKey(key)},
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for offline node, got %d: %s", rec.Code, rec.Body.String())
	}

	mode, err := env.database.GetNodeMode("node-a")
	if err != nil {
		t.Fatalf("get node mode: %v", err)
	}
	if mode != "silent_allow" {
		t.Fatalf("expected node mode to remain silent_allow, got %q", mode)
	}

	rules, err := env.database.GetRules("node-a")
	if err != nil {
		t.Fatalf("get rules: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("expected no persisted rules after failed apply, got %d", len(rules))
	}
}

func TestHandleGenerateRulesApplyBatchesRulesAndSwitchesMode(t *testing.T) {
	env := newAPITestEnv(t)
	env.seedNode(t, "node-a", true)

	env.insertConnection(t, db.Connection{
		Time:        "2026-03-16 10:00:00",
		Node:        "node-a",
		Action:      "allow",
		Protocol:    "tcp",
		SrcIP:       "10.0.0.1",
		SrcPort:     10001,
		DstIP:       "93.184.216.34",
		DstHost:     "example.com",
		DstPort:     443,
		Process:     "/usr/bin/curl",
		ProcessArgs: "--head",
		Rule:        "silent-allow",
	})
	env.insertConnection(t, db.Connection{
		Time:        "2026-03-16 10:30:00",
		Node:        "node-a",
		Action:      "allow",
		Protocol:    "udp",
		SrcIP:       "10.0.0.2",
		SrcPort:     5300,
		DstIP:       "1.1.1.1",
		DstPort:     53,
		Process:     "/usr/bin/dig",
		ProcessArgs: "@1.1.1.1",
		Rule:        "silent-allow",
	})

	previewRec := performJSONRequest(t, env.api.handleGenerateRulesPreview, http.MethodPost, "/api/v1/rules/generate/preview", generateRulesRequest{
		Node:  "node-a",
		Since: "2026-03-16 09:00:00",
		Until: "2026-03-16 12:00:00",
	})
	if previewRec.Code != http.StatusOK {
		t.Fatalf("expected preview 200, got %d: %s", previewRec.Code, previewRec.Body.String())
	}

	preview := decodeJSON[generateRulesPreviewResponse](t, previewRec)
	if len(preview.Data) != 2 {
		t.Fatalf("expected 2 proposals, got %d", len(preview.Data))
	}

	selected := preview.Data[0]
	applyRec := performJSONRequest(t, env.api.handleGenerateRulesApply, http.MethodPost, "/api/v1/rules/generate/apply", applyGeneratedRulesRequest{
		Node:         "node-a",
		Since:        "2026-03-16 09:00:00",
		Until:        "2026-03-16 12:00:00",
		Fingerprints: []string{selected.Fingerprint},
	})
	if applyRec.Code != http.StatusCreated {
		t.Fatalf("expected apply 201, got %d: %s", applyRec.Code, applyRec.Body.String())
	}

	mode, err := env.database.GetNodeMode("node-a")
	if err != nil {
		t.Fatalf("get node mode: %v", err)
	}
	if mode != "ask" {
		t.Fatalf("expected node mode to switch to ask, got %q", mode)
	}

	rules, err := env.database.GetRules("node-a")
	if err != nil {
		t.Fatalf("get rules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 persisted rule, got %d", len(rules))
	}

	persisted, err := buildRuleResponse(&rules[0])
	if err != nil {
		t.Fatalf("build persisted rule response: %v", err)
	}
	if !persisted.IsCompound {
		t.Fatalf("expected persisted rule to remain compound, got %+v", persisted)
	}

	node := env.nodes.GetNode("node-a")
	if node == nil {
		t.Fatal("expected connected node")
	}

	select {
	case notif := <-node.NotifyChan:
		if notif.GetType() != pb.Action_CHANGE_RULE {
			t.Fatalf("expected CHANGE_RULE notification, got %v", notif.GetType())
		}
		if len(notif.GetRules()) != 1 {
			t.Fatalf("expected 1 batched rule, got %d", len(notif.GetRules()))
		}
		if notif.GetRules()[0].GetOperator().GetType() != "list" {
			t.Fatalf("expected compound operator in notification, got %+v", notif.GetRules()[0].GetOperator())
		}
	default:
		t.Fatal("expected queued notification for applied rules")
	}
}

func TestHandleGenerateRulesApplyQueueFullRollsBackState(t *testing.T) {
	env := newAPITestEnv(t)
	env.seedNode(t, "node-a", true)

	env.insertConnection(t, db.Connection{
		Time:        "2026-03-16 10:00:00",
		Node:        "node-a",
		Action:      "allow",
		Protocol:    "tcp",
		SrcIP:       "10.0.0.1",
		SrcPort:     10001,
		DstIP:       "93.184.216.34",
		DstHost:     "example.com",
		DstPort:     443,
		Process:     "/usr/bin/curl",
		ProcessArgs: "--head",
		Rule:        "silent-allow",
	})

	previewRec := performJSONRequest(t, env.api.handleGenerateRulesPreview, http.MethodPost, "/api/v1/rules/generate/preview", generateRulesRequest{
		Node:  "node-a",
		Since: "2026-03-16 09:00:00",
		Until: "2026-03-16 12:00:00",
	})
	if previewRec.Code != http.StatusOK {
		t.Fatalf("expected preview 200, got %d: %s", previewRec.Code, previewRec.Body.String())
	}

	preview := decodeJSON[generateRulesPreviewResponse](t, previewRec)
	if len(preview.Data) != 1 {
		t.Fatalf("expected 1 proposal, got %d", len(preview.Data))
	}

	node := env.nodes.GetNode("node-a")
	if node == nil {
		t.Fatal("expected connected node")
	}
	for i := 0; i < cap(node.NotifyChan); i++ {
		node.NotifyChan <- &pb.Notification{Id: uint64(i + 1)}
	}

	applyRec := performJSONRequest(t, env.api.handleGenerateRulesApply, http.MethodPost, "/api/v1/rules/generate/apply", applyGeneratedRulesRequest{
		Node:         "node-a",
		Since:        "2026-03-16 09:00:00",
		Until:        "2026-03-16 12:00:00",
		Fingerprints: []string{preview.Data[0].Fingerprint},
	})
	if applyRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 when notification queue is full, got %d: %s", applyRec.Code, applyRec.Body.String())
	}

	mode, err := env.database.GetNodeMode("node-a")
	if err != nil {
		t.Fatalf("get node mode: %v", err)
	}
	if mode != "silent_allow" {
		t.Fatalf("expected node mode to roll back to silent_allow, got %q", mode)
	}

	rules, err := env.database.GetRules("node-a")
	if err != nil {
		t.Fatalf("get rules: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("expected no persisted rules after rollback, got %d", len(rules))
	}
}

func TestHandleGetRulesReturnsCompoundRuleRoundTripFromSubscribe(t *testing.T) {
	env := newAPITestEnv(t)
	service := grpcserver.NewUIService(env.nodes, env.database, env.hub, env.prompter)

	compoundRule := ruleutil.BuildGeneratedRule(ruleutil.LearningKey{
		Process:         "/usr/bin/curl",
		DestinationType: "dest.host",
		Destination:     "example.com",
		DstPort:         443,
		Protocol:        "tcp",
	})

	ctx := peer.NewContext(context.Background(), &peer.Peer{
		Addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 50051},
	})

	if _, err := service.Subscribe(ctx, &pb.ClientConfig{
		Name:    "test-node",
		Version: "1.0.0",
		Rules:   []*pb.Rule{compoundRule},
	}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rules?node=127.0.0.1:50051", nil)
	rec := httptest.NewRecorder()
	env.api.handleGetRules(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	response := decodeJSON[[]ruleResponse](t, rec)
	if len(response) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(response))
	}
	if !response[0].IsCompound {
		t.Fatalf("expected compound round-trip, got %+v", response[0])
	}
	if response[0].Operator == nil || response[0].Operator.GetType() != "list" {
		t.Fatalf("expected operator list after round-trip, got %+v", response[0].Operator)
	}
	if len(response[0].Operator.GetList()) != 4 {
		t.Fatalf("expected 4 operator children after round-trip, got %d", len(response[0].Operator.GetList()))
	}
}

func TestHandleUpdateRuleRejectsCompoundRules(t *testing.T) {
	env := newAPITestEnv(t)

	compoundRule := ruleutil.BuildGeneratedRule(ruleutil.LearningKey{
		Process:         "/usr/bin/curl",
		DestinationType: "dest.host",
		Destination:     "example.com",
		DstPort:         443,
		Protocol:        "tcp",
	})
	env.insertRule(t, "node-a", compoundRule)

	rec := performJSONRequestWithRuleName(t, env.api.handleUpdateRule, http.MethodPut, "/api/v1/rules/"+compoundRule.GetName(), compoundRule.GetName(), ruleRequest{
		Node:            "node-a",
		Enabled:         true,
		Action:          "allow",
		Duration:        "always",
		OperatorType:    "simple",
		OperatorOperand: "process.path",
		OperatorData:    "/usr/bin/curl",
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for compound update, got %d: %s", rec.Code, rec.Body.String())
	}
}
