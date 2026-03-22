package api

import (
	"bytes"
	"context"
	"database/sql"
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
	provisionResult *routerpkg.ProvisionResult
	provisionErr    error
	daemonResult    *routerpkg.ProvisionResult
	daemonErr       error
	steps           []routerpkg.ProvisionStep
	err             error
}

func (s *stubRouterProvisioner) Provision(ctx context.Context, req routerpkg.ConnectRequest) (*routerpkg.ProvisionResult, error) {
	return s.provisionResult, s.provisionErr
}

func (s *stubRouterProvisioner) Deprovision(ctx context.Context, addr string, sshPort int, sshUser, sshPass, sshKey string) ([]routerpkg.ProvisionStep, error) {
	return s.steps, s.err
}

func (s *stubRouterProvisioner) CheckCapabilities(ctx context.Context, addr string, sshPort int, sshUser, sshPass, sshKey string) (*routerpkg.CapabilityCheckResult, error) {
	return &routerpkg.CapabilityCheckResult{}, s.err
}

func (s *stubRouterProvisioner) ProvisionDaemon(ctx context.Context, req routerpkg.DaemonRequest) (*routerpkg.ProvisionResult, error) {
	if s.daemonResult != nil || s.daemonErr != nil {
		return s.daemonResult, s.daemonErr
	}
	return &routerpkg.ProvisionResult{Steps: s.steps}, s.err
}

func (s *stubRouterProvisioner) DeprovisionDaemon(ctx context.Context, addr string, sshPort int, sshUser, sshPass, sshKey string) ([]routerpkg.ProvisionStep, error) {
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

func TestHandleConnectRouterRejectsInvalidMode(t *testing.T) {
	env := newAPITestEnv(t)

	rec := performJSONRequest(t, env.api.handleConnectRouter, http.MethodPost, "/api/v1/routers/connect", map[string]string{
		"addr":     "192.168.1.1",
		"ssh_pass": "secret",
		"mode":     "invalid",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleConnectRouterManageFailureFallsBackToMonitor(t *testing.T) {
	env := newAPITestEnv(t)
	env.api.routerProv = &stubRouterProvisioner{
		provisionResult: &routerpkg.ProvisionResult{
			Router: &db.Router{
				Name:       "router-a",
				Addr:       "192.168.1.1",
				SSHPort:    22,
				SSHUser:    "root",
				DaemonMode: db.RouterDaemonModeConntrackAgent,
			},
			Steps: []routerpkg.ProvisionStep{{Step: "connect", Status: "done", Message: "Connected"}},
		},
		daemonResult: &routerpkg.ProvisionResult{
			Router: &db.Router{
				Name:       "router-a",
				Addr:       "192.168.1.1",
				SSHPort:    22,
				SSHUser:    "root",
				DaemonMode: db.RouterDaemonModeConntrackAgent,
			},
			Capabilities: &routerpkg.RouterCapabilities{
				RAMMB:            64,
				RAMSupported:     false,
				IneligibleReason: "router-daemon v1 requires at least 128MB RAM",
			},
			Steps: []routerpkg.ProvisionStep{{Step: "capabilities", Status: "error", Message: "router-daemon v1 requires at least 128MB RAM"}},
		},
		daemonErr: errors.New("router-daemon v1 requires at least 128MB RAM"),
	}

	rec := performJSONRequest(t, env.api.handleConnectRouter, http.MethodPost, "/api/v1/routers/connect", map[string]string{
		"addr":     "192.168.1.1",
		"ssh_pass": "secret",
		"mode":     "manage",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	response := decodeJSON[struct {
		Warning      string                        `json:"warning"`
		Capabilities *routerpkg.RouterCapabilities `json:"capabilities"`
		Steps        []routerpkg.ProvisionStep     `json:"steps"`
		Router       struct {
			DaemonMode string `json:"daemon_mode"`
		} `json:"router"`
	}](t, rec)
	if response.Router.DaemonMode != db.RouterDaemonModeConntrackAgent {
		t.Fatalf("expected router to remain in monitor mode, got %+v", response.Router)
	}
	if response.Warning == "" {
		t.Fatalf("expected warning in response")
	}
	if len(response.Steps) != 2 {
		t.Fatalf("expected combined steps, got %+v", response.Steps)
	}
	if response.Capabilities == nil || response.Capabilities.RAMSupported {
		t.Fatalf("expected capabilities to be returned on manage fallback, got %+v", response.Capabilities)
	}

	node, err := env.database.GetNode("192.168.1.1")
	if err != nil {
		t.Fatalf("expected monitor node record to be created, got %v", err)
	}
	if node.DaemonVersion != "conntrack-agent" {
		t.Fatalf("expected legacy node version to remain conntrack-agent, got %+v", node)
	}
}

func TestHandleConnectRouterManageSuccessSkipsLegacyNodeUpsert(t *testing.T) {
	env := newAPITestEnv(t)
	env.api.routerProv = &stubRouterProvisioner{
		provisionResult: &routerpkg.ProvisionResult{
			Router: &db.Router{
				Name:       "router-a",
				Addr:       "192.168.1.1",
				SSHPort:    22,
				SSHUser:    "root",
				DaemonMode: db.RouterDaemonModeConntrackAgent,
			},
			Steps: []routerpkg.ProvisionStep{{Step: "connect", Status: "done", Message: "Connected"}},
		},
		daemonResult: &routerpkg.ProvisionResult{
			Router: &db.Router{
				Name:       "router-a",
				Addr:       "192.168.1.1",
				SSHPort:    22,
				SSHUser:    "root",
				DaemonMode: db.RouterDaemonModeRouterDaemon,
			},
			Steps: []routerpkg.ProvisionStep{{Step: "verify", Status: "done", Message: "router-daemon subscribed"}},
		},
	}

	rec := performJSONRequest(t, env.api.handleConnectRouter, http.MethodPost, "/api/v1/routers/connect", map[string]string{
		"addr":     "192.168.1.1",
		"ssh_pass": "secret",
		"mode":     "manage",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if _, err := env.database.GetNode("192.168.1.1"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected no legacy node upsert after manage success, got %v", err)
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

func TestHandleDeleteRouterRemovesOfflineRouterAndNode(t *testing.T) {
	env := newAPITestEnv(t)
	seedRouterRecord(t, env, "router-a", time.Now().Add(-2*time.Minute))

	rec := performJSONRequestWithAddr(t, env.api.handleDeleteRouter, http.MethodDelete, "/api/v1/routers/router-a", "router-a", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if _, err := env.database.GetRouterByAddr("router-a"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected router removed, got %v", err)
	}
	if _, err := env.database.GetNode("router-a"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected router node removed, got %v", err)
	}
}

func TestHandleDeleteRouterRejectsOnlineRouter(t *testing.T) {
	env := newAPITestEnv(t)
	seedRouterRecord(t, env, "router-a", time.Now().Add(-30*time.Second))

	rec := performJSONRequestWithAddr(t, env.api.handleDeleteRouter, http.MethodDelete, "/api/v1/routers/router-a", "router-a", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDeleteRouterRemovesManagedRuntimeNode(t *testing.T) {
	env := newAPITestEnv(t)

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

	if err := env.database.UpsertNode(&db.Node{
		Addr:          "managed-node",
		Hostname:      "managed-node",
		DaemonVersion: "opensnitchd-router",
		Status:        "offline",
		LastConn:      time.Now().Add(-2 * time.Minute).Format("2006-01-02 15:04:05"),
	}); err != nil {
		t.Fatalf("seed managed node: %v", err)
	}

	rec := performJSONRequestWithAddr(t, env.api.handleDeleteRouter, http.MethodDelete, "/api/v1/routers/192.168.1.1", "192.168.1.1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if _, err := env.database.GetRouterByAddr("192.168.1.1"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected router removed, got %v", err)
	}
	if _, err := env.database.GetNode("managed-node"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected managed node removed, got %v", err)
	}
}
