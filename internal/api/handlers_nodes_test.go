package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/evilsocket/opensnitch-web/internal/db"
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
