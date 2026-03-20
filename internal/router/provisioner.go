package router

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/bilalbayram/opensnitch-web/internal/db"
	"golang.org/x/crypto/ssh"
)

type remoteClient interface {
	Close() error
	Run(cmd string) (string, error)
	WriteFile(path, content string) error
}

type sshRemoteClient struct {
	client *ssh.Client
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

type Provisioner struct {
	db    *db.Database
	dial  func(addr string, port int, user, pass, key string) (remoteClient, error)
	sleep func(time.Duration)
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
}

type ProvisionResult struct {
	Router          *db.Router      `json:"router"`
	Steps           []ProvisionStep `json:"steps"`
	ServerURL       string          `json:"server_url"`
	ServerURLSource string          `json:"server_url_source"`
}

type ProvisionStep struct {
	Step    string `json:"step"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func NewProvisioner(database *db.Database) *Provisioner {
	return &Provisioner{
		db:    database,
		dial:  sshDial,
		sleep: time.Sleep,
	}
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

	// 3. Install conntrack if needed (OpenWrt package is "conntrack", not "conntrack-tools")
	out, err = client.Run("which conntrack >/dev/null 2>&1 && echo INSTALLED || echo MISSING")
	if err != nil {
		addStep("dependencies", "error", fmt.Sprintf("Failed to check packages: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}
	if strings.TrimSpace(out) == "MISSING" {
		out, err = client.Run("opkg update && opkg install conntrack")
		if err != nil {
			// Retry once — OpenWrt mirrors can be transiently unreachable
			p.sleep(3 * time.Second)
			out, err = client.Run("opkg update && opkg install conntrack")
		}
		if err != nil {
			addStep("dependencies", "error", fmt.Sprintf("Failed to install conntrack (mirror may be unreachable): %s", strings.TrimSpace(out)))
			return &ProvisionResult{Steps: steps}, fmt.Errorf("opkg install failed: %w", err)
		}
		addStep("dependencies", "done", "Installed conntrack")
	} else {
		addStep("dependencies", "done", "conntrack already available")
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
		Name:      req.Name,
		Addr:      req.Addr,
		SSHPort:   req.SSHPort,
		SSHUser:   req.SSHUser,
		APIKey:    apiKey,
		LANSubnet: lanPrefix,
		Status:    "active",
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

// --- helpers ---

func sshDial(addr string, port int, user, pass, key string) (remoteClient, error) {
	var authMethods []ssh.AuthMethod
	if key != "" {
		signer, err := ssh.ParsePrivateKey([]byte(key))
		if err != nil {
			return nil, fmt.Errorf("parse SSH key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if pass != "" {
		authMethods = append(authMethods, ssh.Password(pass))
	}
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", addr, port), config)
	if err != nil {
		return nil, err
	}
	return &sshRemoteClient{client: client}, nil
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
