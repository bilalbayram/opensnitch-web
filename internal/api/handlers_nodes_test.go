package api

import (
	"net/http"
	"testing"
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
