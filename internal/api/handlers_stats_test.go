package api

import (
	"net/http"
	"testing"

	"github.com/bilalbayram/opensnitch-web/internal/db"
	pb "github.com/bilalbayram/opensnitch-web/proto"
)

func TestHandleGetGeneralStatsReturnsDBAlignedSummary(t *testing.T) {
	env := newAPITestEnv(t)
	env.seedNode(t, "node-a", true)

	node := env.nodes.GetNode("node-a")
	if node == nil {
		t.Fatal("expected seeded node to be online")
	}

	node.UpdateStats(&pb.Statistics{
		Connections: 132420,
		Accepted:    132418,
		Dropped:     132417,
		Ignored:     12,
		Rules:       9,
	})

	env.insertConnection(t, db.Connection{
		Time:     "2026-03-16 10:00:00",
		Node:     "node-a",
		Action:   "allow",
		Protocol: "tcp",
		SrcIP:    "10.0.0.1",
		SrcPort:  10001,
		DstIP:    "93.184.216.34",
		DstPort:  443,
		Process:  "/usr/bin/curl",
	})
	env.insertConnection(t, db.Connection{
		Time:     "2026-03-16 10:01:00",
		Node:     "node-a",
		Action:   "deny",
		Protocol: "tcp",
		SrcIP:    "10.0.0.2",
		SrcPort:  10002,
		DstIP:    "203.0.113.10",
		DstPort:  22,
		Process:  "/usr/bin/ssh",
	})
	env.insertConnection(t, db.Connection{
		Time:     "2026-03-16 10:02:00",
		Node:     "node-a",
		Action:   "reject",
		Protocol: "udp",
		SrcIP:    "10.0.0.3",
		SrcPort:  53000,
		DstIP:    "1.1.1.1",
		DstPort:  53,
		Process:  "/usr/bin/dig",
	})

	rec := performJSONRequest(t, env.api.handleGetGeneralStats, http.MethodGet, "/api/v1/stats", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	response := decodeJSON[struct {
		NodesOnline int    `json:"nodes_online"`
		Connections uint64 `json:"connections"`
		Dropped     uint64 `json:"dropped"`
		Accepted    uint64 `json:"accepted"`
		Ignored     uint64 `json:"ignored"`
		Rules       uint64 `json:"rules"`
		WSClients   int    `json:"ws_clients"`
		Total       int64  `json:"total"`
		Allowed     int64  `json:"allowed"`
		Denied      int64  `json:"denied"`
	}](t, rec)

	if response.NodesOnline != 1 {
		t.Fatalf("expected one online node, got %+v", response)
	}
	if response.Connections != 132420 || response.Accepted != 132418 || response.Dropped != 132417 || response.Ignored != 12 {
		t.Fatalf("expected legacy live counters to remain unchanged, got %+v", response)
	}
	if response.Rules != 9 {
		t.Fatalf("expected live rules=9, got %+v", response)
	}
	if response.WSClients != 0 {
		t.Fatalf("expected no websocket clients, got %+v", response)
	}
	if response.Total != 3 || response.Allowed != 1 || response.Denied != 2 {
		t.Fatalf("expected db summary total=3 allowed=1 denied=2, got %+v", response)
	}
	if response.Denied == int64(response.Dropped) {
		t.Fatalf("expected db denied count to differ from daemon dropped count, got %+v", response)
	}
}
