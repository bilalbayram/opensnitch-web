package db_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/bilalbayram/opensnitch-web/internal/db"
)

func newTestDatabase(t *testing.T) *db.Database {
	t.Helper()

	database, err := db.New(filepath.Join(t.TempDir(), "opensnitch-web.db"))
	if err != nil {
		t.Fatalf("create database: %v", err)
	}

	t.Cleanup(func() {
		_ = database.Close()
	})

	return database
}

func insertConnection(t *testing.T, database *db.Database, conn db.Connection) {
	t.Helper()

	if err := database.InsertConnection(&conn); err != nil {
		t.Fatalf("insert connection: %v", err)
	}
}

func TestGetUniqueFlowsGroupsAndFilters(t *testing.T) {
	database := newTestDatabase(t)

	insertConnection(t, database, db.Connection{
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
		ProcessArgs: "--head https://example.com",
		Rule:        "silent-allow",
	})
	insertConnection(t, database, db.Connection{
		Time:        "2026-03-16 11:00:00",
		Node:        "node-a",
		Action:      "allow",
		Protocol:    "tcp",
		SrcIP:       "10.0.0.1",
		SrcPort:     10002,
		DstIP:       "93.184.216.34",
		DstHost:     "example.com",
		DstPort:     443,
		Process:     "/usr/bin/curl",
		ProcessArgs: "--verbose https://example.com",
		Rule:        "silent-allow",
	})
	insertConnection(t, database, db.Connection{
		Time:        "2026-03-16 12:00:00",
		Node:        "node-a",
		Action:      "allow",
		Protocol:    "udp",
		SrcIP:       "10.0.0.2",
		SrcPort:     12000,
		DstIP:       "1.1.1.1",
		DstPort:     53,
		Process:     "/usr/bin/dig",
		ProcessArgs: "@1.1.1.1",
		Rule:        "silent-allow",
	})
	insertConnection(t, database, db.Connection{
		Time:        "2026-03-16 13:00:00",
		Node:        "node-a",
		Action:      "allow",
		Protocol:    "tcp",
		SrcIP:       "10.0.0.3",
		SrcPort:     10003,
		DstIP:       "93.184.216.34",
		DstHost:     "example.com",
		DstPort:     443,
		Process:     "/usr/bin/curl",
		ProcessArgs: "--silent https://example.com",
		Rule:        "manual-rule",
	})
	insertConnection(t, database, db.Connection{
		Time:        "2026-03-16 14:00:00",
		Node:        "node-b",
		Action:      "allow",
		Protocol:    "tcp",
		SrcIP:       "10.0.0.4",
		SrcPort:     10004,
		DstIP:       "93.184.216.34",
		DstHost:     "example.com",
		DstPort:     443,
		Process:     "/usr/bin/curl",
		ProcessArgs: "--retry 1 https://example.com",
		Rule:        "silent-allow",
	})
	insertConnection(t, database, db.Connection{
		Time:        "2026-03-15 09:00:00",
		Node:        "node-a",
		Action:      "allow",
		Protocol:    "tcp",
		SrcIP:       "10.0.0.5",
		SrcPort:     10005,
		DstIP:       "93.184.216.34",
		DstHost:     "example.com",
		DstPort:     443,
		Process:     "/usr/bin/curl",
		ProcessArgs: "--retry 2 https://example.com",
		Rule:        "silent-allow",
	})

	since := time.Date(2026, 3, 16, 9, 30, 0, 0, time.Local)
	until := time.Date(2026, 3, 16, 12, 30, 0, 0, time.Local)

	flows, err := database.GetUniqueFlows("node-a", since, until, nil)
	if err != nil {
		t.Fatalf("get unique flows: %v", err)
	}

	if len(flows) != 2 {
		t.Fatalf("expected 2 flows, got %d", len(flows))
	}

	if flows[0].Process != "/usr/bin/curl" {
		t.Fatalf("expected grouped curl flow first, got %q", flows[0].Process)
	}
	if flows[0].Hits != 2 {
		t.Fatalf("expected curl flow hits=2, got %d", flows[0].Hits)
	}
	if flows[0].DestinationOperand != "dest.host" || flows[0].Destination != "example.com" {
		t.Fatalf("expected host-based destination fallback, got %+v", flows[0])
	}
	if flows[0].FirstSeen != "2026-03-16 10:00:00" || flows[0].LastSeen != "2026-03-16 11:00:00" {
		t.Fatalf("unexpected first/last seen for curl flow: %+v", flows[0])
	}

	if flows[1].DestinationOperand != "dest.ip" || flows[1].Destination != "1.1.1.1" {
		t.Fatalf("expected IP fallback for second flow, got %+v", flows[1])
	}

	filtered, err := database.GetUniqueFlows("node-a", since, until, []string{"/usr/bin/dig"})
	if err != nil {
		t.Fatalf("get filtered flows: %v", err)
	}

	if len(filtered) != 1 {
		t.Fatalf("expected exclusion to remove one flow, got %d", len(filtered))
	}
	if filtered[0].Process != "/usr/bin/curl" {
		t.Fatalf("expected curl flow after exclusion, got %+v", filtered[0])
	}
}

func TestGetConnectionSummaryCountsActions(t *testing.T) {
	database := newTestDatabase(t)

	insertConnection(t, database, db.Connection{
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
	insertConnection(t, database, db.Connection{
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
	insertConnection(t, database, db.Connection{
		Time:     "2026-03-16 10:02:00",
		Node:     "node-b",
		Action:   "reject",
		Protocol: "udp",
		SrcIP:    "10.0.0.3",
		SrcPort:  53000,
		DstIP:    "1.1.1.1",
		DstPort:  53,
		Process:  "/usr/bin/dig",
	})

	summary, err := database.GetConnectionSummary()
	if err != nil {
		t.Fatalf("get connection summary: %v", err)
	}

	if summary.Total != 3 {
		t.Fatalf("expected total=3, got %d", summary.Total)
	}
	if summary.Allowed != 1 {
		t.Fatalf("expected allowed=1, got %d", summary.Allowed)
	}
	if summary.Denied != 2 {
		t.Fatalf("expected denied=2, got %d", summary.Denied)
	}
}

func TestGetConnectionSummaryEmpty(t *testing.T) {
	database := newTestDatabase(t)

	summary, err := database.GetConnectionSummary()
	if err != nil {
		t.Fatalf("get empty connection summary: %v", err)
	}

	if summary.Total != 0 || summary.Allowed != 0 || summary.Denied != 0 {
		t.Fatalf("expected zero summary, got %+v", summary)
	}
}
