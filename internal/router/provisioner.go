package router

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bilalbayram/opensnitch-web/internal/config"
	"github.com/bilalbayram/opensnitch-web/internal/db"
	"github.com/bilalbayram/opensnitch-web/internal/nodemanager"
	"golang.org/x/crypto/ssh"
)

type remoteClient interface {
	Close() error
	Run(cmd string) (string, error)
	WriteFile(path, content string) error
	WriteBinary(path string, content []byte, mode os.FileMode) error
}

type sshRemoteClient struct {
	client *ssh.Client
}

type sshDialError struct {
	msg string
	err error
}

func (e *sshDialError) Error() string {
	return e.msg
}

func (e *sshDialError) Unwrap() error {
	return e.err
}

func (c *sshRemoteClient) Close() error {
	return c.client.Close()
}

func (c *sshRemoteClient) Run(cmd string) (string, error) {
	return runSSHCommand(c.client, cmd)
}

func (c *sshRemoteClient) WriteFile(path, content string) error {
	return writeSSHRemoteFile(c.client, path, content)
}

func (c *sshRemoteClient) WriteBinary(path string, content []byte, mode os.FileMode) error {
	return writeSSHRemoteBinary(c.client, path, content, mode)
}

type Provisioner struct {
	db       *db.Database
	cfg      *config.Config
	nodes    *nodemanager.Manager
	dial     func(addr string, port int, user, pass, key string) (remoteClient, error)
	sleep    func(time.Duration)
	readFile func(string) ([]byte, error)
}

type ConnectRequest struct {
	Addr      string `json:"addr"`
	SSHPort   int    `json:"ssh_port"`
	SSHUser   string `json:"ssh_user"`
	SSHPass   string `json:"ssh_pass"`
	SSHKey    string `json:"ssh_key,omitempty"`
	Name      string `json:"name"`
	LANSubnet string `json:"lan_subnet"`
	ServerURL string `json:"server_url,omitempty"`
	Mode      string `json:"mode,omitempty"`
}

type ProvisionResult struct {
	Router          *db.Router          `json:"router"`
	Capabilities    *RouterCapabilities `json:"capabilities,omitempty"`
	Steps           []ProvisionStep     `json:"steps"`
	ServerURL       string              `json:"server_url"`
	ServerURLSource string              `json:"server_url_source"`
	Warning         string              `json:"warning,omitempty"`
}

type DaemonRequest struct {
	Addr            string `json:"-"`
	SSHPort         int    `json:"ssh_port"`
	SSHUser         string `json:"ssh_user"`
	SSHPass         string `json:"ssh_pass"`
	SSHKey          string `json:"ssh_key,omitempty"`
	NodeName        string `json:"node_name"`
	DefaultAction   string `json:"default_action"`
	PollIntervalMS  int    `json:"poll_interval_ms"`
	FirewallBackend string `json:"firewall_backend"`
}

type ProvisionStep struct {
	Step    string `json:"step"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

const (
	routerDaemonBinaryLocalPath  = "bin/opensnitchd-router-linux-arm64"
	routerDaemonBinaryRemotePath = "/usr/bin/opensnitchd-router"
	routerDaemonConfigDir        = "/etc/opensnitchd-router"
	routerDaemonConfigPath       = "/etc/opensnitchd-router/config.json"
	routerDaemonInitdPath        = "/etc/init.d/opensnitchd-router"

	ConnectModeMonitor = "monitor"
	ConnectModeManage  = "manage"
)

func NewProvisioner(database *db.Database) *Provisioner {
	return &Provisioner{
		db:       database,
		dial:     sshDial,
		sleep:    time.Sleep,
		readFile: os.ReadFile,
	}
}

func NormalizeConnectMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", ConnectModeMonitor:
		return ConnectModeMonitor
	case ConnectModeManage:
		return ConnectModeManage
	default:
		return ""
	}
}

func (p *Provisioner) WithRuntime(nodes *nodemanager.Manager, cfg *config.Config) *Provisioner {
	p.nodes = nodes
	p.cfg = cfg
	return p
}

func (p *Provisioner) Provision(ctx context.Context, req ConnectRequest) (*ProvisionResult, error) {
	var steps []ProvisionStep

	addStep := func(step, status, message string) {
		steps = append(steps, ProvisionStep{Step: step, Status: status, Message: message})
		log.Printf("[router] %s: %s — %s", step, status, message)
	}

	existing, _ := p.db.GetRouterByAddr(req.Addr)

	// 1. SSH connect
	client, err := p.dial(req.Addr, req.SSHPort, req.SSHUser, req.SSHPass, req.SSHKey)
	if err != nil {
		addStep("connect", "error", fmt.Sprintf("SSH connection failed: %v", err))
		return &ProvisionResult{Steps: steps}, fmt.Errorf("ssh dial: %w", err)
	}
	defer client.Close()
	addStep("connect", "done", fmt.Sprintf("Connected to %s:%d", req.Addr, req.SSHPort))

	// 2. Verify OpenWrt
	out, err := client.Run("cat /etc/openwrt_release 2>/dev/null")
	if err != nil || !strings.Contains(out, "DISTRIB") {
		addStep("verify", "error", "Not an OpenWrt device")
		return &ProvisionResult{Steps: steps}, fmt.Errorf("not an OpenWrt device")
	}
	addStep("verify", "done", "OpenWrt verified")

	// 3. Install conntrack + GNU wget if needed.
	// BusyBox wget lacks --header support, so the agent's POST calls fail on stock OpenWrt.
	out, err = client.Run("{ which conntrack && wget --version 2>&1 | grep -q GNU; } >/dev/null 2>&1 && echo INSTALLED || echo MISSING")
	if err != nil {
		addStep("dependencies", "error", fmt.Sprintf("Failed to check packages: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}
	if strings.TrimSpace(out) == "MISSING" {
		out, err = client.Run("opkg update && opkg install conntrack wget")
		if err != nil {
			// Retry once — OpenWrt mirrors can be transiently unreachable
			p.sleep(3 * time.Second)
			out, err = client.Run("opkg update && opkg install conntrack wget")
		}
		if err != nil {
			addStep("dependencies", "error", fmt.Sprintf("Failed to install packages (mirror may be unreachable): %s", strings.TrimSpace(out)))
			return &ProvisionResult{Steps: steps}, fmt.Errorf("opkg install failed: %w", err)
		}
		addStep("dependencies", "done", "Installed conntrack and wget")
	} else {
		addStep("dependencies", "done", "conntrack and GNU wget already available")
	}

	apiKey := ""
	if existing != nil {
		apiKey = existing.APIKey
	} else {
		apiKey, err = db.GenerateAPIKey()
		if err != nil {
			addStep("deploy", "error", fmt.Sprintf("Failed to generate API key: %v", err))
			return &ProvisionResult{Steps: steps}, err
		}
	}

	lanPrefix := deriveLANPrefix(req.Addr, req.LANSubnet)

	agentCfg := RenderAgentConfig(AgentConfig{
		ServerURL:  req.ServerURL,
		APIKey:     apiKey,
		RouterName: req.Name,
		LANPrefix:  lanPrefix,
	})

	// 5. Deploy files
	if _, err := client.Run("mkdir -p /etc/conntrack-agent"); err != nil {
		addStep("deploy", "error", fmt.Sprintf("Failed to create directory: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}

	if err := client.WriteFile("/etc/conntrack-agent/config", agentCfg); err != nil {
		addStep("deploy", "error", fmt.Sprintf("Failed to write config: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}

	if err := client.WriteFile("/etc/conntrack-agent/agent.sh", AgentScript()); err != nil {
		addStep("deploy", "error", fmt.Sprintf("Failed to write agent script: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}

	if err := client.WriteFile("/etc/init.d/conntrack-agent", InitdScript()); err != nil {
		addStep("deploy", "error", fmt.Sprintf("Failed to write init.d script: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}

	if _, err := client.Run("chmod +x /etc/conntrack-agent/agent.sh /etc/init.d/conntrack-agent"); err != nil {
		addStep("deploy", "error", fmt.Sprintf("Failed to chmod: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}
	addStep("deploy", "done", "Agent files deployed")

	// 6. Enable and start service
	if _, err := client.Run("/etc/init.d/conntrack-agent enable"); err != nil {
		addStep("start", "error", fmt.Sprintf("Failed to enable service: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}

	if _, err := client.Run("/etc/init.d/conntrack-agent start"); err != nil {
		addStep("start", "error", fmt.Sprintf("Failed to start service: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}

	// Brief pause then verify
	p.sleep(time.Second)
	out, err = client.Run("pgrep -f 'conntrack-agent/agent.sh' >/dev/null 2>&1 && echo RUNNING || echo STOPPED")
	if err != nil || strings.TrimSpace(out) != "RUNNING" {
		addStep("start", "error", "Service started but process not found")
		return &ProvisionResult{Steps: steps}, fmt.Errorf("agent not running after start")
	}
	addStep("start", "done", "Service running")

	// 7. Verify the agent can reach the server
	out, err = client.Run(fmt.Sprintf("wget -q -O /dev/null --timeout=5 %q 2>&1 || echo UNREACHABLE", req.ServerURL+"/api/v1/ingest"))
	if err != nil || strings.Contains(out, "UNREACHABLE") {
		addStep("connectivity", "warning", fmt.Sprintf("Router cannot reach server at %s — check firewall rules (ingest port must be open to router network)", req.ServerURL))
	} else {
		addStep("connectivity", "done", "Server reachable from router")
	}

	// 8. Store router record
	router := &db.Router{
		Name:           req.Name,
		Addr:           req.Addr,
		SSHPort:        req.SSHPort,
		SSHUser:        req.SSHUser,
		APIKey:         apiKey,
		LANSubnet:      lanPrefix,
		DaemonMode:     db.RouterDaemonModeConntrackAgent,
		LinkedNodeAddr: "",
		Status:         "active",
	}
	if err := p.db.UpsertRouter(router); err != nil {
		addStep("register", "error", fmt.Sprintf("Failed to save router: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}
	addStep("register", "done", "Router registered")

	return &ProvisionResult{Router: router, Steps: steps}, nil
}

func (p *Provisioner) Deprovision(ctx context.Context, addr string, sshPort int, sshUser, sshPass, sshKey string) ([]ProvisionStep, error) {
	var steps []ProvisionStep

	client, err := p.dial(addr, sshPort, sshUser, sshPass, sshKey)
	if err != nil {
		steps = append(steps, ProvisionStep{"connect", "error", fmt.Sprintf("SSH failed: %v", err)})
		return steps, err
	}
	defer client.Close()
	steps = append(steps, ProvisionStep{"connect", "done", "Connected"})

	if out, err := client.Run("if [ -x /etc/init.d/conntrack-agent ]; then /etc/init.d/conntrack-agent stop; else echo ABSENT; fi"); err != nil {
		steps = append(steps, ProvisionStep{"stop", "error", strings.TrimSpace(out)})
		return steps, fmt.Errorf("stop agent: %w", err)
	} else {
		msg := "Agent stopped"
		if strings.TrimSpace(out) == "ABSENT" {
			msg = "Agent service already absent"
		}
		steps = append(steps, ProvisionStep{"stop", "done", msg})
	}

	if out, err := client.Run("if [ -x /etc/init.d/conntrack-agent ]; then /etc/init.d/conntrack-agent disable; else echo ABSENT; fi"); err != nil {
		steps = append(steps, ProvisionStep{"disable", "error", strings.TrimSpace(out)})
		return steps, fmt.Errorf("disable agent: %w", err)
	} else {
		msg := "Agent disabled"
		if strings.TrimSpace(out) == "ABSENT" {
			msg = "Agent service already absent"
		}
		steps = append(steps, ProvisionStep{"disable", "done", msg})
	}

	if out, err := client.Run("rm -rf /etc/conntrack-agent /etc/init.d/conntrack-agent"); err != nil {
		steps = append(steps, ProvisionStep{"remove", "error", strings.TrimSpace(out)})
		return steps, fmt.Errorf("remove agent files: %w", err)
	}
	steps = append(steps, ProvisionStep{"remove", "done", "Agent removed"})

	return steps, nil
}

func (p *Provisioner) CheckCapabilities(ctx context.Context, addr string, sshPort int, sshUser, sshPass, sshKey string) (*CapabilityCheckResult, error) {
	_ = ctx

	steps := []ProvisionStep{}
	addStep := func(step, status, message string) {
		steps = append(steps, ProvisionStep{Step: step, Status: status, Message: message})
		log.Printf("[router] %s: %s — %s", step, status, message)
	}

	client, err := p.dial(addr, sshPort, sshUser, sshPass, sshKey)
	if err != nil {
		addStep("connect", "error", fmt.Sprintf("SSH connection failed: %v", err))
		return &CapabilityCheckResult{Steps: steps}, err
	}
	defer client.Close()
	addStep("connect", "done", fmt.Sprintf("Connected to %s:%d", addr, sshPort))

	caps, err := CheckCapabilities(client)
	if err != nil {
		addStep("capabilities", "error", err.Error())
		return &CapabilityCheckResult{Capabilities: caps, Steps: steps}, err
	}

	status := "done"
	message := "Router is eligible for router-daemon v1"
	if !caps.Eligible {
		status = "error"
		message = caps.IneligibleReason
	}
	addStep("capabilities", status, message)

	return &CapabilityCheckResult{Capabilities: caps, Steps: steps}, nil
}

func (p *Provisioner) ProvisionDaemon(ctx context.Context, req DaemonRequest) (*ProvisionResult, error) {
	p.applyDaemonDefaults(&req)

	steps := []ProvisionStep{}
	addStep := func(step, status, message string) {
		steps = append(steps, ProvisionStep{Step: step, Status: status, Message: message})
		log.Printf("[router] %s: %s — %s", step, status, message)
	}

	existing, err := p.db.GetRouterByAddr(req.Addr)
	if err != nil {
		addStep("lookup", "error", "Router must be connected before upgrading to router-daemon")
		return &ProvisionResult{Steps: steps}, err
	}

	client, err := p.dial(req.Addr, req.SSHPort, req.SSHUser, req.SSHPass, req.SSHKey)
	if err != nil {
		addStep("connect", "error", fmt.Sprintf("SSH connection failed: %v", err))
		return &ProvisionResult{Steps: steps}, fmt.Errorf("ssh dial: %w", err)
	}
	defer client.Close()
	addStep("connect", "done", fmt.Sprintf("Connected to %s:%d", req.Addr, req.SSHPort))

	caps, err := CheckCapabilities(client)
	if err != nil {
		addStep("capabilities", "error", err.Error())
		return &ProvisionResult{Router: existing, Capabilities: caps, Steps: steps}, err
	}
	if !caps.Eligible {
		addStep("capabilities", "error", caps.IneligibleReason)
		return &ProvisionResult{Router: existing, Capabilities: caps, Steps: steps}, fmt.Errorf(caps.IneligibleReason)
	}
	addStep("capabilities", "done", "Router is eligible for router-daemon v1")

	grpcAddr, grpcSource, err := p.resolveGRPCPublicAddr(req.Addr)
	if err != nil {
		addStep("config", "error", err.Error())
		return &ProvisionResult{Router: existing, Capabilities: caps, Steps: steps}, err
	}
	addStep("config", "done", fmt.Sprintf("Using public gRPC address %s (%s)", grpcAddr, grpcSource))

	binary, err := p.readFile(filepath.Clean(routerDaemonBinaryLocalPath))
	if err != nil {
		addStep("deploy", "error", fmt.Sprintf("Missing local router-daemon binary at %s", routerDaemonBinaryLocalPath))
		return &ProvisionResult{Router: existing, Capabilities: caps, Steps: steps}, err
	}

	daemonConfig, err := RenderDaemonConfig(DaemonConfig{
		GRPCAddr:        grpcAddr,
		APIKey:          existing.APIKey,
		NodeName:        req.NodeName,
		DefaultAction:   req.DefaultAction,
		PollIntervalMS:  req.PollIntervalMS,
		FirewallBackend: req.FirewallBackend,
	})
	if err != nil {
		addStep("config", "error", fmt.Sprintf("Render daemon config: %v", err))
		return &ProvisionResult{Router: existing, Capabilities: caps, Steps: steps}, err
	}

	stoppedLegacy := false
	if out, err := client.Run("if [ -x /etc/init.d/conntrack-agent ]; then /etc/init.d/conntrack-agent stop && /etc/init.d/conntrack-agent disable; else echo ABSENT; fi"); err != nil {
		addStep("stop_legacy", "error", strings.TrimSpace(out))
		return &ProvisionResult{Router: existing, Capabilities: caps, Steps: steps}, fmt.Errorf("stop conntrack-agent: %w", err)
	} else {
		if strings.TrimSpace(out) == "ABSENT" {
			addStep("stop_legacy", "done", "Legacy conntrack-agent already absent")
		} else {
			stoppedLegacy = true
			addStep("stop_legacy", "done", "Stopped and disabled legacy conntrack-agent")
		}
	}

	rollbackLegacy := func(reason string) {
		if !stoppedLegacy {
			return
		}
		if _, rollbackErr := client.Run("if [ -x /etc/init.d/conntrack-agent ]; then /etc/init.d/conntrack-agent enable && /etc/init.d/conntrack-agent start; fi"); rollbackErr != nil {
			addStep("rollback", "warning", fmt.Sprintf("Failed to restore conntrack-agent after %s: %v", reason, rollbackErr))
			return
		}
		addStep("rollback", "done", fmt.Sprintf("Restored conntrack-agent after %s", reason))
	}

	if _, err := client.Run("mkdir -p " + shellQuote(routerDaemonConfigDir)); err != nil {
		addStep("deploy", "error", fmt.Sprintf("Create daemon config dir: %v", err))
		rollbackLegacy("directory creation failure")
		return &ProvisionResult{Router: existing, Capabilities: caps, Steps: steps}, err
	}
	if err := client.WriteBinary(routerDaemonBinaryRemotePath, binary, 0755); err != nil {
		addStep("deploy", "error", fmt.Sprintf("Upload router-daemon binary: %v", err))
		rollbackLegacy("binary upload failure")
		return &ProvisionResult{Router: existing, Capabilities: caps, Steps: steps}, err
	}
	if err := client.WriteFile(routerDaemonConfigPath, daemonConfig); err != nil {
		addStep("deploy", "error", fmt.Sprintf("Write daemon config: %v", err))
		rollbackLegacy("config upload failure")
		return &ProvisionResult{Router: existing, Capabilities: caps, Steps: steps}, err
	}
	if err := client.WriteFile(routerDaemonInitdPath, DaemonInitdScript()); err != nil {
		addStep("deploy", "error", fmt.Sprintf("Write daemon init script: %v", err))
		rollbackLegacy("init script upload failure")
		return &ProvisionResult{Router: existing, Capabilities: caps, Steps: steps}, err
	}
	if _, err := client.Run("chmod +x " + shellQuote(routerDaemonInitdPath)); err != nil {
		addStep("deploy", "error", fmt.Sprintf("chmod daemon init script: %v", err))
		rollbackLegacy("init script chmod failure")
		return &ProvisionResult{Router: existing, Capabilities: caps, Steps: steps}, err
	}
	addStep("deploy", "done", "opensnitchd-router files deployed")

	if _, err := client.Run(shellQuote(routerDaemonInitdPath) + " enable"); err != nil {
		addStep("start", "error", fmt.Sprintf("Enable router-daemon service: %v", err))
		rollbackLegacy("enable failure")
		return &ProvisionResult{Router: existing, Capabilities: caps, Steps: steps}, err
	}
	if _, err := client.Run(shellQuote(routerDaemonInitdPath) + " start"); err != nil {
		addStep("start", "error", fmt.Sprintf("Start router-daemon service: %v", err))
		rollbackLegacy("start failure")
		return &ProvisionResult{Router: existing, Capabilities: caps, Steps: steps}, err
	}
	addStep("start", "done", "router-daemon service started")

	if err := p.waitForManagedNode(ctx, req.Addr, 20*time.Second); err != nil {
		addStep("verify", "error", fmt.Sprintf("router-daemon did not subscribe as %s: %v", req.Addr, err))
		if cleanupErr := p.removeDaemonArtifacts(client); cleanupErr != nil {
			addStep("rollback", "warning", fmt.Sprintf("Failed to remove router-daemon after subscribe failure: %v", cleanupErr))
		} else {
			addStep("rollback", "done", "Removed router-daemon after failed verification")
		}
		rollbackLegacy("verification failure")
		_ = p.db.UpdateRouterRuntime(existing.Addr, db.RouterDaemonModeConntrackAgent, "")
		return &ProvisionResult{Router: existing, Capabilities: caps, Steps: steps}, err
	}
	addStep("verify", "done", fmt.Sprintf("router-daemon subscribed as %s", req.Addr))

	if err := p.db.UpdateRouterRuntime(existing.Addr, db.RouterDaemonModeRouterDaemon, existing.Addr); err != nil {
		addStep("register", "error", fmt.Sprintf("Update router runtime: %v", err))
		if cleanupErr := p.removeDaemonArtifacts(client); cleanupErr != nil {
			addStep("rollback", "warning", fmt.Sprintf("Failed to remove router-daemon after DB update failure: %v", cleanupErr))
		}
		rollbackLegacy("runtime update failure")
		return &ProvisionResult{Router: existing, Capabilities: caps, Steps: steps}, err
	}
	if err := p.db.UpdateRouterStatus(existing.Addr, "active"); err != nil {
		addStep("register", "warning", fmt.Sprintf("Router-daemon connected but status update failed: %v", err))
	} else {
		addStep("register", "done", "Router runtime updated to router-daemon")
	}

	updated, err := p.db.GetRouterByAddr(existing.Addr)
	if err != nil {
		updated = existing
	}
	return &ProvisionResult{Router: updated, Capabilities: caps, Steps: steps}, nil
}

func (p *Provisioner) DeprovisionDaemon(ctx context.Context, addr string, sshPort int, sshUser, sshPass, sshKey string) ([]ProvisionStep, error) {
	_ = ctx

	steps := []ProvisionStep{}
	client, err := p.dial(addr, sshPort, sshUser, sshPass, sshKey)
	if err != nil {
		steps = append(steps, ProvisionStep{"connect", "error", fmt.Sprintf("SSH failed: %v", err)})
		return steps, err
	}
	defer client.Close()
	steps = append(steps, ProvisionStep{"connect", "done", "Connected"})

	if err := p.removeDaemonArtifacts(client); err != nil {
		steps = append(steps, ProvisionStep{"remove", "error", err.Error()})
		return steps, err
	}

	steps = append(steps,
		ProvisionStep{"stop", "done", "router-daemon stopped"},
		ProvisionStep{"remove", "done", "router-daemon removed and nft state flushed"},
	)
	return steps, nil
}

// --- helpers ---

func sshDial(addr string, port int, user, pass, key string) (remoteClient, error) {
	target := net.JoinHostPort(addr, strconv.Itoa(port))
	var authMethods []ssh.AuthMethod
	if key != "" {
		signer, err := ssh.ParsePrivateKey([]byte(key))
		if err != nil {
			return nil, &sshDialError{
				msg: "SSH private key is invalid or unsupported",
				err: fmt.Errorf("parse SSH key: %w", err),
			}
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if pass != "" {
		authMethods = append(authMethods, ssh.Password(pass))
		authMethods = append(authMethods, ssh.KeyboardInteractive(
			func(_ string, _ string, questions []string, _ []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = pass
				}
				return answers, nil
			},
		))
	}
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	const timeout = 10 * time.Second
	conn, err := net.DialTimeout("tcp", target, timeout)
	if err != nil {
		return nil, wrapSSHDialError(addr, port, user, err)
	}
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("set SSH deadline: %w", err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, target, config)
	if err != nil {
		_ = conn.Close()
		return nil, wrapSSHDialError(addr, port, user, err)
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		_ = sshConn.Close()
		return nil, fmt.Errorf("clear SSH deadline: %w", err)
	}

	return &sshRemoteClient{client: ssh.NewClient(sshConn, chans, reqs)}, nil
}

func wrapSSHDialError(addr string, port int, user string, err error) error {
	if err == nil {
		return nil
	}
	return &sshDialError{
		msg: explainSSHDialError(addr, port, user, err),
		err: err,
	}
}

func explainSSHDialError(addr string, port int, user string, err error) string {
	target := net.JoinHostPort(addr, strconv.Itoa(port))
	message := strings.TrimSpace(err.Error())
	lower := strings.ToLower(message)

	switch {
	case isSSHTimeoutOrUnreachable(err, lower):
		return fmt.Sprintf("router is offline or unreachable at %s", target)
	case errors.Is(err, syscall.ECONNREFUSED) || strings.Contains(lower, "connection refused"):
		return fmt.Sprintf("router is reachable at %s, but SSH is not accepting connections on that port", target)
	case strings.Contains(lower, "unable to authenticate"):
		return fmt.Sprintf("SSH authentication failed for %s@%s. Verify the SSH username and password or key", user, target)
	case strings.Contains(lower, "connection reset by peer"), strings.Contains(lower, "broken pipe"), strings.Contains(lower, "unexpected packet"), strings.Contains(lower, "eof"):
		return fmt.Sprintf("router responded at %s, but the SSH handshake did not complete", target)
	default:
		return fmt.Sprintf("SSH connection to %s failed: %s", target, message)
	}
}

func isSSHTimeoutOrUnreachable(err error, lower string) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return errors.Is(err, syscall.ETIMEDOUT) ||
		errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETUNREACH) ||
		strings.Contains(lower, "i/o timeout") ||
		strings.Contains(lower, "no route to host") ||
		strings.Contains(lower, "host is down") ||
		strings.Contains(lower, "network is unreachable")
}

func runSSHCommand(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(cmd)
	output := stdout.String() + stderr.String()
	return output, err
}

func writeSSHRemoteFile(client *ssh.Client, path, content string) error {
	// Use heredoc with a delimiter that won't appear in our controlled content.
	// Single-quoted delimiter ('AGENT_EOF') prevents any shell expansion.
	const delim = "___COLOMBO_EOF___"
	cmd := fmt.Sprintf("cat > %s <<'%s'\n%s\n%s", path, delim, content, delim)
	out, err := runSSHCommand(client, cmd)
	if err != nil {
		return fmt.Errorf("%w (output: %s)", err, strings.TrimSpace(out))
	}
	return nil
}

func writeSSHRemoteBinary(client *ssh.Client, path string, content []byte, mode os.FileMode) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	session.Stdin = bytes.NewReader(content)

	cmd := fmt.Sprintf("cat > %s && chmod %o %s", shellQuote(path), mode.Perm(), shellQuote(path))
	if err := session.Run(cmd); err != nil {
		output := stdout.String() + stderr.String()
		return fmt.Errorf("%w (output: %s)", err, strings.TrimSpace(output))
	}
	return nil
}

func (p *Provisioner) applyDaemonDefaults(req *DaemonRequest) {
	if req.SSHPort == 0 {
		req.SSHPort = 22
	}
	if req.SSHUser == "" {
		req.SSHUser = "root"
	}
	if req.NodeName == "" {
		req.NodeName = req.Addr
	}
	if req.DefaultAction == "" {
		req.DefaultAction = "deny"
		if p.cfg != nil && strings.TrimSpace(p.cfg.UI.DefaultAction) != "" {
			req.DefaultAction = p.cfg.UI.DefaultAction
		}
	}
	if req.PollIntervalMS <= 0 {
		req.PollIntervalMS = 1000
	}
	if req.FirewallBackend == "" {
		req.FirewallBackend = "nft"
	}
}

func (p *Provisioner) waitForManagedNode(ctx context.Context, addr string, timeout time.Duration) error {
	if p.nodes == nil {
		return fmt.Errorf("router-daemon verification requires node manager runtime")
	}

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		if p.nodes.GetNode(addr) != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("timed out waiting for authenticated subscribe")
		case <-ticker.C:
		}
	}
}

func (p *Provisioner) removeDaemonArtifacts(client remoteClient) error {
	stopCmd := "if [ -x " + shellQuote(routerDaemonInitdPath) + " ]; then " + shellQuote(routerDaemonInitdPath) + " stop; " + shellQuote(routerDaemonInitdPath) + " disable; fi"
	if _, err := client.Run(stopCmd); err != nil {
		return fmt.Errorf("stop router-daemon: %w", err)
	}
	if _, err := client.Run("nft delete table inet opensnitch-router >/dev/null 2>&1 || true"); err != nil {
		return fmt.Errorf("flush router-daemon nft table: %w", err)
	}
	if _, err := client.Run("rm -rf " + shellQuote(routerDaemonConfigDir) + " " + shellQuote(routerDaemonInitdPath) + " " + shellQuote(routerDaemonBinaryRemotePath)); err != nil {
		return fmt.Errorf("remove router-daemon files: %w", err)
	}
	return nil
}

func (p *Provisioner) resolveGRPCPublicAddr(routerAddr string) (string, string, error) {
	if p.cfg == nil {
		return "", "", fmt.Errorf("set server.grpc_public_addr to upgrade routers to router-daemon")
	}

	if public := strings.TrimSpace(p.cfg.Server.GRPCPublicAddr); public != "" {
		return public, "config_override", nil
	}

	host, port, err := net.SplitHostPort(strings.TrimSpace(p.cfg.Server.GRPCAddr))
	if err != nil {
		return "", "", fmt.Errorf("invalid server.grpc_addr %q", p.cfg.Server.GRPCAddr)
	}
	if host != "" && host != "0.0.0.0" && host != "::" && host != "[::]" {
		return net.JoinHostPort(host, port), "listen_addr", nil
	}

	lanURL, source := ResolveServerURL(routerAddr, p.cfg.Server.HTTPAddr)
	if lanURL == "" {
		return "", "", fmt.Errorf("could not infer public gRPC address from server config; set server.grpc_public_addr")
	}

	parsed, err := url.Parse(lanURL)
	if err != nil || parsed.Hostname() == "" {
		return "", "", fmt.Errorf("invalid inferred LAN URL %q; set server.grpc_public_addr", lanURL)
	}

	return net.JoinHostPort(parsed.Hostname(), port), source, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func deriveLANPrefix(routerAddr, userSubnet string) string {
	if userSubnet != "" {
		// If user provided CIDR like 192.168.1.0/24, derive the prefix
		if strings.Contains(userSubnet, "/") {
			ip, _, err := net.ParseCIDR(userSubnet)
			if err == nil {
				parts := strings.Split(ip.String(), ".")
				if len(parts) == 4 {
					return parts[0] + "." + parts[1] + "." + parts[2] + "."
				}
			}
		}
		return userSubnet
	}

	// Auto-derive from router IP: 192.168.1.1 → 192.168.1.
	parts := strings.Split(routerAddr, ".")
	if len(parts) == 4 {
		return parts[0] + "." + parts[1] + "." + parts[2] + "."
	}
	return "192.168.1."
}
