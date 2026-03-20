package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/bilalbayram/opensnitch-web/internal/db"
	routerpkg "github.com/bilalbayram/opensnitch-web/internal/router"
)

type stubRouterProvisioner struct {
	steps []routerpkg.ProvisionStep
	err   error
}

func (s *stubRouterProvisioner) Provision(ctx context.Context, req routerpkg.ConnectRequest) (*routerpkg.ProvisionResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRouterProvisioner) Deprovision(ctx context.Context, addr string, sshPort int, sshUser, sshPass, sshKey string) ([]routerpkg.ProvisionStep, error) {
	return s.steps, s.err
}

func seedRouterRecord(t *testing.T, env *apiTestEnv, addr string, lastConn time.Time) {
	t.Helper()

	if err := env.database.InsertRouter(&db.Router{
		Name:      addr,
		Addr:      addr,
		SSHPort:   22,
		SSHUser:   "root",
		APIKey:    "api-key-" + addr,
		LANSubnet: "192.168.1.0/24",
		Status:    "connected",
	}); err != nil {
		t.Fatalf("seed router: %v", err)
	}

	if err := env.database.UpsertNode(&db.Node{
		Addr:          addr,
		Hostname:      addr,
		DaemonVersion: "conntrack-agent",
		Status:        "online",
		LastConn:      lastConn.Format("2006-01-02 15:04:05"),
		SourceType:    "router",
	}); err != nil {
		t.Fatalf("seed router node: %v", err)
	}
}

func performJSONRequestWithAddr(t *testing.T, handler http.HandlerFunc, method, target, addr string, payload any) *httptest.ResponseRecorder {
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
	routeCtx.URLParams.Add("addr", addr)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))

	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func TestHandleGetRoutersMarksRecentHeartbeatOnline(t *testing.T) {
	env := newAPITestEnv(t)
	seedRouterRecord(t, env, "router-a", time.Now().Add(-30*time.Second))

	rec := performJSONRequest(t, env.api.handleGetRouters, http.MethodGet, "/api/v1/routers", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	response := decodeJSON[[]struct {
		Addr   string `json:"addr"`
		Online bool   `json:"online"`
	}](t, rec)
	if len(response) != 1 {
		t.Fatalf("expected 1 router, got %d", len(response))
	}
	if response[0].Addr != "router-a" {
		t.Fatalf("unexpected router payload: %+v", response[0])
	}
	if !response[0].Online {
		t.Fatalf("expected router to be online, got %+v", response[0])
	}
}

func TestHandleGetRoutersMarksStaleHeartbeatOffline(t *testing.T) {
	env := newAPITestEnv(t)
	seedRouterRecord(t, env, "router-a", time.Now().Add(-2*time.Minute))

	rec := performJSONRequest(t, env.api.handleGetRouters, http.MethodGet, "/api/v1/routers", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	response := decodeJSON[[]struct {
		Online bool `json:"online"`
	}](t, rec)
	if len(response) != 1 {
		t.Fatalf("expected 1 router, got %d", len(response))
	}
	if response[0].Online {
		t.Fatalf("expected router to be offline, got %+v", response[0])
	}
}

func TestHandleGetRoutersOmitsAPIKey(t *testing.T) {
	env := newAPITestEnv(t)
	seedRouterRecord(t, env, "router-a", time.Now().Add(-30*time.Second))

	rec := performJSONRequest(t, env.api.handleGetRouters, http.MethodGet, "/api/v1/routers", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	response := decodeJSON[[]map[string]any](t, rec)
	if len(response) != 1 {
		t.Fatalf("expected 1 router, got %d", len(response))
	}
	if _, ok := response[0]["api_key"]; ok {
		t.Fatalf("expected api_key to be omitted, got %+v", response[0])
	}
}

func TestHandleDisconnectRouterFailurePreservesState(t *testing.T) {
	env := newAPITestEnv(t)
	env.api.routerProv = &stubRouterProvisioner{
		steps: []routerpkg.ProvisionStep{{Step: "connect", Status: "error", Message: "bad password"}},
		err:   errors.New("ssh failed"),
	}
	seedRouterRecord(t, env, "router-a", time.Now().Add(-30*time.Second))

	rec := performJSONRequestWithAddr(t, env.api.handleDisconnectRouter, http.MethodPost, "/api/v1/routers/router-a/disconnect", "router-a", map[string]string{
		"ssh_pass": "wrong",
	})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}

	if _, err := env.database.GetRouterByAddr("router-a"); err != nil {
		t.Fatalf("expected router record to remain, got %v", err)
	}

	node, err := env.database.GetNode("router-a")
	if err != nil {
		t.Fatalf("expected router node to remain, got %v", err)
	}
	if node.Status != "online" {
		t.Fatalf("expected node status online, got %q", node.Status)
	}
}

func TestHandleDisconnectRouterSuccessRemovesState(t *testing.T) {
	env := newAPITestEnv(t)
	env.api.routerProv = &stubRouterProvisioner{
		steps: []routerpkg.ProvisionStep{{Step: "remove", Status: "done", Message: "Agent removed"}},
	}
	seedRouterRecord(t, env, "router-a", time.Now().Add(-30*time.Second))

	rec := performJSONRequestWithAddr(t, env.api.handleDisconnectRouter, http.MethodPost, "/api/v1/routers/router-a/disconnect", "router-a", map[string]string{
		"ssh_pass": "secret",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if env.database.IsRouterAddr("router-a") {
		t.Fatalf("expected router record to be removed")
	}

	node, err := env.database.GetNode("router-a")
	if err != nil {
		t.Fatalf("expected node record to remain, got %v", err)
	}
	if node.Status != "offline" {
		t.Fatalf("expected node status offline, got %q", node.Status)
	}
}
