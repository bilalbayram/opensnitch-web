package router

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/bilalbayram/opensnitch-web/internal/db"
	"golang.org/x/crypto/ssh"
)

type timeoutNetError struct{}

func (timeoutNetError) Error() string   { return "i/o timeout" }
func (timeoutNetError) Timeout() bool   { return true }
func (timeoutNetError) Temporary() bool { return true }

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
			"cat /etc/openwrt_release 2>/dev/null": {output: "DISTRIB_ID='OpenWrt'\n"},
			"{ which conntrack && wget --version 2>&1 | grep -q GNU; } >/dev/null 2>&1 && echo INSTALLED || echo MISSING": {output: "INSTALLED\n"},
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
		// Allow connectivity check commands to pass by default
		if strings.HasPrefix(cmd, "wget -q -O /dev/null --timeout=5") {
			return "", nil
		}
		return "", fmt.Errorf("unexpected command: %s", cmd)
	}
	return result.output, result.err
}

func (c *fakeRemoteClient) WriteFile(path, content string) error {
	c.writes[path] = content
	return nil
}

func (c *fakeRemoteClient) WriteBinary(path string, content []byte, mode os.FileMode) error {
	c.writes[path] = fmt.Sprintf("binary:%d:%o", len(content), mode.Perm())
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
		dial: func(addr string, port int, user, pass, key string) (remoteClient, error) {
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

func TestProvisionOpkgRetryOnTransientFailure(t *testing.T) {
	client := newFakeRemoteClient()

	// First opkg call fails, second succeeds
	client.outputs["{ which conntrack && wget --version 2>&1 | grep -q GNU; } >/dev/null 2>&1 && echo INSTALLED || echo MISSING"] = fakeRunResult{output: "MISSING\n"}

	callCount := 0
	prov, _ := newTestProvisioner(t, &fakeRemoteClientWithOpkgRetry{
		fakeRemoteClient: client,
		opkgCallCount:    &callCount,
	})

	_, err := prov.Provision(context.Background(), ConnectRequest{
		Addr:      "192.168.1.1",
		SSHPort:   22,
		SSHUser:   "root",
		SSHPass:   "secret",
		Name:      "router-retry",
		LANSubnet: "192.168.1.0/24",
		ServerURL: "http://server:8080",
	})
	if err != nil {
		t.Fatalf("provision should succeed after retry: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected opkg to be called twice (initial + retry), got %d", callCount)
	}
}

// fakeRemoteClientWithOpkgRetry wraps fakeRemoteClient to fail the first opkg call
type fakeRemoteClientWithOpkgRetry struct {
	*fakeRemoteClient
	opkgCallCount *int
}

func (c *fakeRemoteClientWithOpkgRetry) Run(cmd string) (string, error) {
	if cmd == "opkg update && opkg install conntrack wget" {
		*c.opkgCallCount++
		if *c.opkgCallCount == 1 {
			return "mirror unreachable", errors.New("opkg failed")
		}
		return "installed\n", nil
	}
	return c.fakeRemoteClient.Run(cmd)
}

func TestProvisionConnectivityWarningOnUnreachableServer(t *testing.T) {
	client := newFakeRemoteClient()
	// Override the connectivity check to simulate failure
	prov, _ := newTestProvisioner(t, &fakeRemoteClientWithConnCheck{
		fakeRemoteClient: client,
		connCheckFails:   true,
	})

	result, err := prov.Provision(context.Background(), ConnectRequest{
		Addr:      "192.168.1.1",
		SSHPort:   22,
		SSHUser:   "root",
		SSHPass:   "secret",
		Name:      "router-warn",
		LANSubnet: "192.168.1.0/24",
		ServerURL: "http://server:8080",
	})
	if err != nil {
		t.Fatalf("provision should succeed even when connectivity check fails: %v", err)
	}

	// Find the connectivity step
	found := false
	for _, step := range result.Steps {
		if step.Step == "connectivity" {
			found = true
			if step.Status != "warning" {
				t.Fatalf("expected connectivity step status=warning, got %q", step.Status)
			}
			if !strings.Contains(step.Message, "cannot reach server") {
				t.Fatalf("expected warning message about unreachable server, got %q", step.Message)
			}
		}
	}
	if !found {
		t.Fatalf("expected connectivity step in results, got steps: %+v", result.Steps)
	}
}

type fakeRemoteClientWithConnCheck struct {
	*fakeRemoteClient
	connCheckFails bool
}

func (c *fakeRemoteClientWithConnCheck) Run(cmd string) (string, error) {
	if strings.HasPrefix(cmd, "wget -q -O /dev/null --timeout=5") {
		if c.connCheckFails {
			return "UNREACHABLE\n", nil
		}
		return "", nil
	}
	return c.fakeRemoteClient.Run(cmd)
}

func TestSSHDialSupportsKeyboardInteractivePasswordAuth(t *testing.T) {
	signer, err := testSigner()
	if err != nil {
		t.Fatalf("create host key: %v", err)
	}

	serverConfig := &ssh.ServerConfig{
		KeyboardInteractiveCallback: func(conn ssh.ConnMetadata, challenge ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			answers, err := challenge(conn.User(), "keyboard-interactive", []string{"Password: "}, []bool{false})
			if err != nil {
				return nil, err
			}
			if len(answers) != 1 || answers[0] != "secret" {
				return nil, errors.New("bad password")
			}
			return nil, nil
		},
	}
	serverConfig.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()

		_, chans, reqs, err := ssh.NewServerConn(conn, serverConfig)
		if err != nil {
			done <- err
			return
		}
		go ssh.DiscardRequests(reqs)
		for ch := range chans {
			ch.Reject(ssh.UnknownChannelType, "unsupported")
		}
		done <- nil
	}()

	addr := listener.Addr().(*net.TCPAddr)
	client, err := sshDial("127.0.0.1", addr.Port, "root", "secret", "")
	if err != nil {
		t.Fatalf("ssh dial: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("server handshake: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for keyboard-interactive handshake")
	}
}

func TestExplainSSHDialErrorPasswordOnlyAuth(t *testing.T) {
	err := errors.New("ssh: handshake failed: ssh: unable to authenticate, attempted methods [none password], no supported methods remain")

	message := explainSSHDialError("192.168.1.1", 22, "root", err)

	if !strings.Contains(message, "SSH authentication failed for root@192.168.1.1:22") {
		t.Fatalf("expected auth failure explanation, got %q", message)
	}
}

func TestExplainSSHDialErrorOfflineTimeout(t *testing.T) {
	err := &net.OpError{Op: "dial", Net: "tcp", Err: timeoutNetError{}}

	message := explainSSHDialError("192.168.1.1", 22, "root", err)

	if message != "router is offline or unreachable at 192.168.1.1:22" {
		t.Fatalf("unexpected message: %q", message)
	}
}

func TestExplainSSHDialErrorConnectionRefused(t *testing.T) {
	err := &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}

	message := explainSSHDialError("192.168.1.1", 22, "root", err)

	if message != "router is reachable at 192.168.1.1:22, but SSH is not accepting connections on that port" {
		t.Fatalf("unexpected message: %q", message)
	}
}

func testSigner() (ssh.Signer, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return ssh.NewSignerFromKey(privateKey)
}
