package db_test

import (
	"testing"

	"github.com/bilalbayram/opensnitch-web/internal/db"
)

func TestIncrementNodeCons(t *testing.T) {
	database := newTestDatabase(t)

	// Seed a router node with 5 existing connections
	if err := database.UpsertNode(&db.Node{
		Addr:     "10.0.0.1",
		Hostname: "router-a",
		Cons:     5,
		Status:   db.NodeStatusOffline,
		LastConn: "2025-01-01 00:00:00",
	}); err != nil {
		t.Fatalf("seed node: %v", err)
	}

	// Increment by 3
	if err := database.IncrementNodeCons("10.0.0.1", 3, "2025-06-15 12:00:00"); err != nil {
		t.Fatalf("increment cons: %v", err)
	}

	node, err := database.GetNode("10.0.0.1")
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if node.Cons != 8 {
		t.Fatalf("expected cons=8, got %d", node.Cons)
	}
	if node.Status != db.NodeStatusOnline {
		t.Fatalf("expected status=online, got %q", node.Status)
	}
	if node.LastConn != "2025-06-15 12:00:00" {
		t.Fatalf("expected last_connection updated, got %q", node.LastConn)
	}
}

func TestUpsertRouterNodePreservesCons(t *testing.T) {
	database := newTestDatabase(t)

	// Seed a node with 42 connections
	if err := database.UpsertNode(&db.Node{
		Addr:       "10.0.0.2",
		Hostname:   "router-b",
		Cons:       42,
		Status:     db.NodeStatusOnline,
		LastConn:   "2025-01-01 00:00:00",
		SourceType: "router",
	}); err != nil {
		t.Fatalf("seed node: %v", err)
	}

	// UpsertRouterNode should NOT reset cons
	if err := database.UpsertRouterNode("10.0.0.2", "router-b-renamed", "conntrack-agent", db.NodeStatusOnline, "2025-06-15 13:00:00"); err != nil {
		t.Fatalf("upsert router node: %v", err)
	}

	node, err := database.GetNode("10.0.0.2")
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if node.Cons != 42 {
		t.Fatalf("expected cons to be preserved at 42, got %d", node.Cons)
	}
	if node.Hostname != "router-b-renamed" {
		t.Fatalf("expected hostname updated, got %q", node.Hostname)
	}
	if node.LastConn != "2025-06-15 13:00:00" {
		t.Fatalf("expected last_connection updated, got %q", node.LastConn)
	}
	if node.SourceType != "router" {
		t.Fatalf("expected source_type=router, got %q", node.SourceType)
	}
}

func TestUpsertRouterNodeCreatesNewNode(t *testing.T) {
	database := newTestDatabase(t)

	// Should create a fresh node with cons=0
	if err := database.UpsertRouterNode("10.0.0.3", "new-router", "conntrack-agent", db.NodeStatusOnline, "2025-06-15 14:00:00"); err != nil {
		t.Fatalf("upsert router node: %v", err)
	}

	node, err := database.GetNode("10.0.0.3")
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if node.Cons != 0 {
		t.Fatalf("expected cons=0 for new node, got %d", node.Cons)
	}
	if node.Hostname != "new-router" {
		t.Fatalf("expected hostname=new-router, got %q", node.Hostname)
	}
	if node.SourceType != "router" {
		t.Fatalf("expected source_type=router, got %q", node.SourceType)
	}
}
