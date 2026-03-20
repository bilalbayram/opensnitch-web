package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/bilalbayram/opensnitch-web/internal/db"
)

func TestHandleGetSeenFlowsSupportsFiltersAndPagination(t *testing.T) {
	env := newAPITestEnv(t)

	entries := []struct {
		key    db.SeenFlowKey
		action string
		when   time.Time
	}{
		{
			key: db.SeenFlowKey{
				Node:               "node-a",
				Process:            "/usr/bin/curl",
				Protocol:           "tcp",
				DstPort:            443,
				DestinationOperand: "dest.host",
				Destination:        "example.com",
			},
			action: "allow",
			when:   time.Date(2026, 3, 16, 10, 0, 0, 0, time.Local),
		},
		{
			key: db.SeenFlowKey{
				Node:               "node-a",
				Process:            "/usr/bin/python3",
				Protocol:           "tcp",
				DstPort:            443,
				DestinationOperand: "dest.host",
				Destination:        "pypi.org",
			},
			action: "deny",
			when:   time.Date(2026, 3, 16, 11, 0, 0, 0, time.Local),
		},
		{
			key: db.SeenFlowKey{
				Node:               "node-b",
				Process:            "/usr/bin/dig",
				Protocol:           "udp",
				DstPort:            53,
				DestinationOperand: "dest.ip",
				Destination:        "1.1.1.1",
			},
			action: "reject",
			when:   time.Date(2026, 3, 16, 12, 0, 0, 0, time.Local),
		},
	}

	for _, entry := range entries {
		if err := env.database.UpsertSeenFlow(entry.key, entry.action, "", entry.when, time.Time{}); err != nil {
			t.Fatalf("seed seen flow: %v", err)
		}
	}

	rec := performJSONRequest(t, env.api.handleGetSeenFlows, http.MethodGet, "/api/v1/seen-flows?node=node-a&action=deny&search=python&limit=10&offset=0", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	response := decodeJSON[struct {
		Data  []db.SeenFlow `json:"data"`
		Total int           `json:"total"`
	}](t, rec)
	if response.Total != 1 || len(response.Data) != 1 {
		t.Fatalf("expected one filtered flow, got total=%d len=%d", response.Total, len(response.Data))
	}
	if response.Data[0].Process != "/usr/bin/python3" {
		t.Fatalf("expected python flow, got %+v", response.Data[0])
	}

	rec = performJSONRequest(t, env.api.handleGetSeenFlows, http.MethodGet, "/api/v1/seen-flows?limit=1&offset=1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	response = decodeJSON[struct {
		Data  []db.SeenFlow `json:"data"`
		Total int           `json:"total"`
	}](t, rec)
	if response.Total != 3 {
		t.Fatalf("expected total=3, got %d", response.Total)
	}
	if len(response.Data) != 1 {
		t.Fatalf("expected one paged flow, got %d", len(response.Data))
	}
	if response.Data[0].Process != "/usr/bin/python3" {
		t.Fatalf("expected second-most-recent flow, got %+v", response.Data[0])
	}
}
