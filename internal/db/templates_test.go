package db

import (
	"path/filepath"
	"testing"
)

func TestReplaceNodeTagsNormalizesAndDeduplicates(t *testing.T) {
	database, err := New(filepath.Join(t.TempDir(), "opensnitch-web.db"))
	if err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	if err := database.UpsertNode(&Node{
		Addr:     "node-a",
		Hostname: "node-a",
		Status:   "offline",
	}); err != nil {
		t.Fatalf("seed node: %v", err)
	}

	tags, err := database.ReplaceNodeTags("node-a", []string{" Server ", "server", "Prod Web", "prod-web"})
	if err != nil {
		t.Fatalf("replace node tags: %v", err)
	}

	if len(tags) != 2 || tags[0] != "server" || tags[1] != "prod-web" {
		t.Fatalf("unexpected normalized tags: %+v", tags)
	}

	stored, err := database.GetNodeTags("node-a")
	if err != nil {
		t.Fatalf("get node tags: %v", err)
	}

	if len(stored) != 2 || stored[0] != "prod-web" || stored[1] != "server" {
		t.Fatalf("unexpected stored tags: %+v", stored)
	}
}

func TestNodeTemplateSyncRoundTrip(t *testing.T) {
	database, err := New(filepath.Join(t.TempDir(), "opensnitch-web.db"))
	if err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	if err := database.SetNodeTemplateSync("node-a", true, "node offline"); err != nil {
		t.Fatalf("set node template sync: %v", err)
	}

	state, err := database.GetNodeTemplateSync("node-a")
	if err != nil {
		t.Fatalf("get node template sync: %v", err)
	}
	if !state.Pending || state.Error != "node offline" {
		t.Fatalf("unexpected sync state: %+v", state)
	}

	if err := database.SetNodeTemplateSync("node-a", false, ""); err != nil {
		t.Fatalf("clear node template sync: %v", err)
	}
	state, err = database.GetNodeTemplateSync("node-a")
	if err != nil {
		t.Fatalf("get node template sync after clear: %v", err)
	}
	if state.Pending || state.Error != "" {
		t.Fatalf("unexpected cleared sync state: %+v", state)
	}
}
