package db_test

import (
	"testing"
	"time"

	"github.com/bilalbayram/opensnitch-web/internal/db"
)

func TestSeenFlowUpsertAndLookup(t *testing.T) {
	database := newTestDatabase(t)

	key := db.SeenFlowKey{
		Node:               "node-a",
		Process:            "/usr/bin/curl",
		Protocol:           "tcp",
		DstPort:            443,
		DestinationOperand: "dest.host",
		Destination:        "example.com",
	}

	firstSeen := time.Date(2026, 3, 16, 10, 0, 0, 0, time.Local)
	secondSeen := firstSeen.Add(15 * time.Minute)

	if err := database.UpsertSeenFlow(key, "allow", "web-rule-1", firstSeen, time.Time{}); err != nil {
		t.Fatalf("insert seen flow: %v", err)
	}

	flow, err := database.GetSeenFlow(key)
	if err != nil {
		t.Fatalf("get seen flow: %v", err)
	}
	if flow == nil {
		t.Fatal("expected seen flow to exist")
	}
	if flow.Count != 1 {
		t.Fatalf("expected count=1, got %d", flow.Count)
	}
	if flow.Action != "allow" {
		t.Fatalf("expected allow action, got %q", flow.Action)
	}
	if flow.SourceRuleName != "web-rule-1" {
		t.Fatalf("expected source rule name to round-trip, got %q", flow.SourceRuleName)
	}
	if flow.FirstSeen != "2026-03-16 10:00:00" || flow.LastSeen != "2026-03-16 10:00:00" {
		t.Fatalf("unexpected timestamps after insert: %+v", flow)
	}

	if err := database.UpsertSeenFlow(key, "deny", "web-rule-2", secondSeen, time.Time{}); err != nil {
		t.Fatalf("update seen flow: %v", err)
	}

	flow, err = database.GetSeenFlow(key)
	if err != nil {
		t.Fatalf("get updated seen flow: %v", err)
	}
	if flow.Count != 2 {
		t.Fatalf("expected count=2, got %d", flow.Count)
	}
	if flow.Action != "deny" {
		t.Fatalf("expected action overwrite to deny, got %q", flow.Action)
	}
	if flow.SourceRuleName != "web-rule-2" {
		t.Fatalf("expected updated source rule name, got %q", flow.SourceRuleName)
	}
	if flow.FirstSeen != "2026-03-16 10:00:00" {
		t.Fatalf("expected first_seen to remain unchanged, got %q", flow.FirstSeen)
	}
	if flow.LastSeen != "2026-03-16 10:15:00" {
		t.Fatalf("expected last_seen to update, got %q", flow.LastSeen)
	}
}

func TestSeenFlowsRemainNodeScoped(t *testing.T) {
	database := newTestDatabase(t)

	keyA := db.SeenFlowKey{
		Node:               "node-a",
		Process:            "/usr/bin/curl",
		Protocol:           "tcp",
		DstPort:            443,
		DestinationOperand: "dest.host",
		Destination:        "example.com",
	}
	keyB := keyA
	keyB.Node = "node-b"

	if err := database.UpsertSeenFlow(keyA, "allow", "web-rule-a", time.Date(2026, 3, 16, 10, 0, 0, 0, time.Local), time.Time{}); err != nil {
		t.Fatalf("insert node-a flow: %v", err)
	}
	if err := database.UpsertSeenFlow(keyB, "deny", "web-rule-b", time.Date(2026, 3, 16, 11, 0, 0, 0, time.Local), time.Time{}); err != nil {
		t.Fatalf("insert node-b flow: %v", err)
	}

	flowA, err := database.GetSeenFlow(keyA)
	if err != nil {
		t.Fatalf("get node-a flow: %v", err)
	}
	flowB, err := database.GetSeenFlow(keyB)
	if err != nil {
		t.Fatalf("get node-b flow: %v", err)
	}

	if flowA == nil || flowB == nil {
		t.Fatal("expected both node-scoped flows to exist")
	}
	if flowA.Action != "allow" || flowB.Action != "deny" {
		t.Fatalf("expected node-specific actions, got node-a=%q node-b=%q", flowA.Action, flowB.Action)
	}
}

func TestGetSeenFlowsSupportsFilteringAndPagination(t *testing.T) {
	database := newTestDatabase(t)

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
		if err := database.UpsertSeenFlow(entry.key, entry.action, "", entry.when, time.Time{}); err != nil {
			t.Fatalf("seed seen flow: %v", err)
		}
	}

	filtered, total, err := database.GetSeenFlows(&db.SeenFlowFilter{
		Node:   "node-a",
		Action: "deny",
		Search: "python",
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("get filtered seen flows: %v", err)
	}
	if total != 1 || len(filtered) != 1 {
		t.Fatalf("expected one filtered seen flow, got total=%d len=%d", total, len(filtered))
	}
	if filtered[0].Process != "/usr/bin/python3" {
		t.Fatalf("expected python flow, got %+v", filtered[0])
	}

	paged, total, err := database.GetSeenFlows(&db.SeenFlowFilter{
		Limit:  1,
		Offset: 1,
	})
	if err != nil {
		t.Fatalf("get paged seen flows: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total=3, got %d", total)
	}
	if len(paged) != 1 {
		t.Fatalf("expected one paged result, got %d", len(paged))
	}
	if paged[0].Process != "/usr/bin/python3" {
		t.Fatalf("expected second-most-recent flow, got %+v", paged[0])
	}
}

func TestSeenFlowTracksExpiry(t *testing.T) {
	database := newTestDatabase(t)

	key := db.SeenFlowKey{
		Node:               "node-a",
		Process:            "/usr/bin/curl",
		Protocol:           "tcp",
		DstPort:            443,
		DestinationOperand: "dest.host",
		Destination:        "example.com",
	}

	now := time.Date(2026, 3, 16, 10, 0, 0, 0, time.Local)
	expiresAt := now.Add(15 * time.Minute)
	if err := database.UpsertSeenFlow(key, "allow", "web-rule-1", now, expiresAt); err != nil {
		t.Fatalf("insert expiring seen flow: %v", err)
	}

	flow, err := database.GetSeenFlow(key)
	if err != nil {
		t.Fatalf("get expiring seen flow: %v", err)
	}
	if flow == nil {
		t.Fatal("expected expiring seen flow to exist")
	}
	if flow.ExpiresAt != "2026-03-16 10:15:00" {
		t.Fatalf("expected expires_at to round-trip, got %q", flow.ExpiresAt)
	}
	if flow.IsExpired(now.Add(10 * time.Minute)) {
		t.Fatal("expected seen flow to remain active before expiry")
	}
	if !flow.IsExpired(now.Add(16 * time.Minute)) {
		t.Fatal("expected seen flow to expire after its deadline")
	}
}

func TestDeleteSeenFlowsBySourceRule(t *testing.T) {
	database := newTestDatabase(t)

	keyA := db.SeenFlowKey{
		Node:               "node-a",
		Process:            "/usr/bin/curl",
		Protocol:           "tcp",
		DstPort:            443,
		DestinationOperand: "dest.host",
		Destination:        "example.com",
	}
	keyB := db.SeenFlowKey{
		Node:               "node-a",
		Process:            "/usr/bin/dig",
		Protocol:           "udp",
		DstPort:            53,
		DestinationOperand: "dest.ip",
		Destination:        "1.1.1.1",
	}

	when := time.Date(2026, 3, 16, 10, 0, 0, 0, time.Local)
	if err := database.UpsertSeenFlow(keyA, "allow", "web-rule-1", when, time.Time{}); err != nil {
		t.Fatalf("insert seen flow A: %v", err)
	}
	if err := database.UpsertSeenFlow(keyB, "allow", "web-rule-2", when, time.Time{}); err != nil {
		t.Fatalf("insert seen flow B: %v", err)
	}

	if err := database.DeleteSeenFlowsBySourceRule("node-a", "web-rule-1"); err != nil {
		t.Fatalf("delete seen flows by source rule: %v", err)
	}

	flowA, err := database.GetSeenFlow(keyA)
	if err != nil {
		t.Fatalf("get deleted seen flow: %v", err)
	}
	if flowA != nil {
		t.Fatalf("expected seen flow A to be deleted, got %+v", flowA)
	}

	flowB, err := database.GetSeenFlow(keyB)
	if err != nil {
		t.Fatalf("get retained seen flow: %v", err)
	}
	if flowB == nil {
		t.Fatal("expected seen flow B to remain")
	}
}
