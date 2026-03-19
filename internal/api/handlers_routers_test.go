package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/evilsocket/opensnitch-web/internal/db"
)

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
