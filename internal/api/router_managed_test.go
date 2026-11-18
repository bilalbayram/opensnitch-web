package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/bilalbayram/opensnitch-web/internal/db"
	"github.com/bilalbayram/opensnitch-web/internal/prompter"
	pb "github.com/bilalbayram/opensnitch-web/proto"
)

func performJSONRequestWithPromptID(t *testing.T, handler http.HandlerFunc, method, target, promptID string, payload any) *httptest.ResponseRecorder {
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
	routeCtx.URLParams.Add("id", promptID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))

	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func seedManagedRouterNode(t *testing.T, env *apiTestEnv, addr string) {
	t.Helper()

	env.seedNode(t, addr, true)
	if err := env.database.InsertRouter(&db.Router{
		Name:           addr,
		Addr:           addr,
		SSHPort:        22,
		SSHUser:        "root",
		APIKey:         "router-key-" + addr,
		LANSubnet:      "192.168.1.0/24",
		DaemonMode:     db.RouterDaemonModeRouterDaemon,
		LinkedNodeAddr: addr,
		Status:         "active",
	}); err != nil {
		t.Fatalf("insert router: %v", err)
	}
}

func TestHandleGetPendingPromptsMarksRouterManaged(t *testing.T) {
	env := newAPITestEnv(t)
	seedManagedRouterNode(t, env, "router-a")

	promptCh := make(chan *prompter.PendingPrompt, 1)
	env.prompter.OnNewPrompt = func(prompt *prompter.PendingPrompt) {
		promptCh <- prompt
	}

	go func() {
		_, _ = env.prompter.AskUser("router-a", &pb.Connection{
			ProcessPath: "/usr/bin/curl",
			DstHost:     "example.com",
			DstIp:       "93.184.216.34",
			DstPort:     443,
			Protocol:    "tcp",
		})
	}()

	prompt := <-promptCh
	rec := performJSONRequest(t, env.api.handleGetPendingPrompts, http.MethodGet, "/api/v1/prompts/pending", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	response := decodeJSON[[]struct {
		ID            string `json:"id"`
		RouterManaged bool   `json:"router_managed"`
	}](t, rec)
	if len(response) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(response))
	}
	if response[0].ID != prompt.ID || !response[0].RouterManaged {
		t.Fatalf("unexpected prompt payload: %+v", response[0])
	}

	if err := env.prompter.Reply(prompt.ID, &pb.Rule{
		Name:     "cleanup",
		Action:   "allow",
		Duration: "once",
		Enabled:  true,
		Operator: &pb.Operator{Type: "simple", Operand: "process.path", Data: "/usr/bin/curl"},
	}); err != nil {
		t.Fatalf("cleanup reply: %v", err)
	}
}

func TestHandlePromptReplyRejectsUnsupportedRouterManagedOperand(t *testing.T) {
	env := newAPITestEnv(t)
	seedManagedRouterNode(t, env, "router-a")

	promptCh := make(chan *prompter.PendingPrompt, 1)
	env.prompter.OnNewPrompt = func(prompt *prompter.PendingPrompt) {
		promptCh <- prompt
	}

	go func() {
		_, _ = env.prompter.AskUser("router-a", &pb.Connection{
			ProcessPath: "/usr/bin/curl",
			DstHost:     "example.com",
			DstIp:       "93.184.216.34",
			DstPort:     443,
			Protocol:    "tcp",
		})
	}()

	prompt := <-promptCh
	rec := performJSONRequestWithPromptID(t, env.api.handlePromptReply, http.MethodPost, "/api/v1/prompts/"+prompt.ID+"/reply", prompt.ID, promptReplyRequest{
		Action:   "deny",
		Duration: "always",
		Name:     "router-rule",
		Operand:  "dest.host",
		Data:     "example.com",
		Operator: "simple",
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := env.prompter.Reply(prompt.ID, &pb.Rule{
		Name:     "cleanup",
		Action:   "allow",
		Duration: "once",
		Enabled:  true,
		Operator: &pb.Operator{Type: "simple", Operand: "process.path", Data: "/usr/bin/curl"},
	}); err != nil {
		t.Fatalf("cleanup reply: %v", err)
	}
}

func TestHandleSetDNSPolicyRejectsUnsupportedRouterManagedHostRules(t *testing.T) {
	env := newAPITestEnv(t)
	seedManagedRouterNode(t, env, "router-a")

	rec := performJSONRequest(t, env.api.handleSetDNSPolicy, http.MethodPost, "/api/v1/dns/policy", dnsPolicyRequest{
		Node:              "router-a",
		Enabled:           true,
		AllowedResolvers:  []string{"1.1.1.1"},
		BlockDoHHostnames: true,
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateRuleRejectsUnsupportedRouterManagedOperand(t *testing.T) {
	env := newAPITestEnv(t)
	seedManagedRouterNode(t, env, "router-a")

	rec := performJSONRequest(t, env.api.handleCreateRule, http.MethodPost, "/api/v1/rules", ruleRequest{
		Name:            "router-host-rule",
		Node:            "router-a",
		Enabled:         true,
		Action:          "deny",
		Duration:        "always",
		OperatorType:    "simple",
		OperatorOperand: "dest.host",
		OperatorData:    "example.com",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlePromptReplyAllowsSupportedRouterManagedOperand(t *testing.T) {
	env := newAPITestEnv(t)
	seedManagedRouterNode(t, env, "router-a")

	promptCh := make(chan *prompter.PendingPrompt, 1)
	resultCh := make(chan *prompter.AskResult, 1)
	env.prompter.OnNewPrompt = func(prompt *prompter.PendingPrompt) {
		promptCh <- prompt
	}

	go func() {
		result, _ := env.prompter.AskUser("router-a", &pb.Connection{
			ProcessPath: "/usr/bin/curl",
			DstIp:       "93.184.216.34",
			DstPort:     443,
			Protocol:    "tcp",
		})
		resultCh <- result
	}()

	prompt := <-promptCh
	rec := performJSONRequestWithPromptID(t, env.api.handlePromptReply, http.MethodPost, "/api/v1/prompts/"+prompt.ID+"/reply", prompt.ID, promptReplyRequest{
		Action:   "deny",
		Duration: "always",
		Name:     "router-ip-rule",
		Operand:  "dest.ip",
		Data:     "93.184.216.34",
		Operator: "simple",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	select {
	case result := <-resultCh:
		if result == nil || result.Rule == nil || result.Rule.GetOperator().GetOperand() != "dest.ip" {
			t.Fatalf("unexpected prompt result: %+v", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for prompt result")
	}
}
