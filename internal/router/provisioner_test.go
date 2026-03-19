package router

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/evilsocket/opensnitch-web/internal/db"
)

type fakeRemoteClient struct {
	outputs map[string]fakeRunResult
	writes  map[string]string
}

type fakeRunResult struct {
	output string
	err    error
}

func newFakeRemoteClient() *fakeRemoteClient {
	return &fakeRemoteClient{
		outputs: map[string]fakeRunResult{
			"cat /etc/openwrt_release 2>/dev/null":                                                {output: "DISTRIB_ID='OpenWrt'\n"},
			"which conntrack >/dev/null 2>&1 && echo INSTALLED || echo MISSING":                   {output: "INSTALLED\n"},
			"mkdir -p /etc/conntrack-agent":                                                       {},
			"chmod +x /etc/conntrack-agent/agent.sh /etc/init.d/conntrack-agent":                  {},
			"/etc/init.d/conntrack-agent enable":                                                  {},
			"/etc/init.d/conntrack-agent start":                                                   {},
			"pgrep -f 'conntrack-agent/agent.sh' >/dev/null 2>&1 && echo RUNNING || echo STOPPED": {output: "RUNNING\n"},
		},
		writes: make(map[string]string),
	}
}

func (c *fakeRemoteClient) Close() error {
	return nil
}

func (c *fakeRemoteClient) Run(cmd string) (string, error) {
	result, ok := c.outputs[cmd]
	if !ok {
		return "", fmt.Errorf("unexpected command: %s", cmd)
	}
	return result.output, result.err
}

func (c *fakeRemoteClient) WriteFile(path, content string) error {
	c.writes[path] = content
	return nil
}

func newTestProvisioner(t *testing.T, client remoteClient) (*Provisioner, *db.Database) {
	t.Helper()

	database, err := db.New(filepath.Join(t.TempDir(), "opensnitch-web.db"))
	if err != nil {
		t.Fatalf("create database: %v", err)
	}

	t.Cleanup(func() {
		_ = database.Close()
	})

	return &Provisioner{
		db: database,
		dial: func(addr string, port int, user, pass string) (remoteClient, error) {
			return client, nil
		},
		sleep: func(_ time.Duration) {},
	}, database
}

func TestProvisionCreatesFreshAPIKey(t *testing.T) {
	client := newFakeRemoteClient()
	prov, database := newTestProvisioner(t, client)

	result, err := prov.Provision(context.Background(), ConnectRequest{
		Addr:      "192.168.1.1",
		SSHPort:   22,
		SSHUser:   "root",
		SSHPass:   "secret",
		Name:      "router-a",
		LANSubnet: "192.168.1.0/24",
		ServerURL: "http://server:8080",
	})
	if err != nil {
		t.Fatalf("provision router: %v", err)
	}
	if result.Router.APIKey == "" {
		t.Fatalf("expected API key to be generated")
	}

	stored, err := database.GetRouterByAddr("192.168.1.1")
	if err != nil {
		t.Fatalf("load router: %v", err)
	}
	if stored.APIKey != result.Router.APIKey {
		t.Fatalf("expected stored API key %q, got %q", result.Router.APIKey, stored.APIKey)
	}

	config := client.writes["/etc/conntrack-agent/config"]
	if !strings.Contains(config, fmt.Sprintf("API_KEY=\"%s\"", result.Router.APIKey)) {
		t.Fatalf("expected config to contain generated API key, got %q", config)
	}
}

func TestProvisionReusesExistingAPIKeyAndUpdatesMetadata(t *testing.T) {
	client := newFakeRemoteClient()
	prov, database := newTestProvisioner(t, client)

	existing := &db.Router{
		Name:      "old-name",
		Addr:      "192.168.1.1",
		SSHPort:   22,
		SSHUser:   "root",
		APIKey:    "existing-key",
		LANSubnet: "192.168.1.",
		Status:    "active",
	}
	if err := database.InsertRouter(existing); err != nil {
		t.Fatalf("seed router: %v", err)
	}

	result, err := prov.Provision(context.Background(), ConnectRequest{
		Addr:      "192.168.1.1",
		SSHPort:   2222,
		SSHUser:   "admin",
		SSHPass:   "secret",
		Name:      "new-name",
		LANSubnet: "192.168.2.0/24",
		ServerURL: "http://server:8080",
	})
	if err != nil {
		t.Fatalf("reprovision router: %v", err)
	}
	if result.Router.APIKey != "existing-key" {
		t.Fatalf("expected existing API key to be reused, got %q", result.Router.APIKey)
	}

	stored, err := database.GetRouterByAddr("192.168.1.1")
	if err != nil {
		t.Fatalf("load router: %v", err)
	}
	if stored.ID != existing.ID {
		t.Fatalf("expected router row to be updated in place, got old ID %d new ID %d", existing.ID, stored.ID)
	}
	if stored.APIKey != "existing-key" {
		t.Fatalf("expected existing API key to remain, got %q", stored.APIKey)
	}
	if stored.Name != "new-name" || stored.SSHPort != 2222 || stored.SSHUser != "admin" || stored.LANSubnet != "192.168.2." {
		t.Fatalf("expected updated router metadata, got %+v", stored)
	}

	config := client.writes["/etc/conntrack-agent/config"]
	if !strings.Contains(config, "API_KEY=\"existing-key\"") {
		t.Fatalf("expected config to reuse existing API key, got %q", config)
	}
}
