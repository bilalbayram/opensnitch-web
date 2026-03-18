package router

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/evilsocket/opensnitch-web/internal/db"
	"golang.org/x/crypto/ssh"
)

type Provisioner struct {
	db *db.Database
}

type ConnectRequest struct {
	Addr      string `json:"addr"`
	SSHPort   int    `json:"ssh_port"`
	SSHUser   string `json:"ssh_user"`
	SSHPass   string `json:"ssh_pass"`
	Name      string `json:"name"`
	LANSubnet string `json:"lan_subnet"`
	ServerURL string `json:"-"`
}

type ProvisionResult struct {
	Router *db.Router      `json:"router"`
	Steps  []ProvisionStep `json:"steps"`
}

type ProvisionStep struct {
	Step    string `json:"step"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func NewProvisioner(database *db.Database) *Provisioner {
	return &Provisioner{db: database}
}

func (p *Provisioner) Provision(ctx context.Context, req ConnectRequest) (*ProvisionResult, error) {
	var steps []ProvisionStep

	addStep := func(step, status, message string) {
		steps = append(steps, ProvisionStep{Step: step, Status: status, Message: message})
		log.Printf("[router] %s: %s — %s", step, status, message)
	}

	// Check if this router is already registered
	existing, _ := p.db.GetRouterByAddr(req.Addr)
	if existing != nil {
		// Delete old record so re-provisioning works cleanly
		p.db.DeleteRouter(req.Addr)
		log.Printf("[router] Re-provisioning %s (old record removed)", req.Addr)
	}

	// 1. SSH connect
	client, err := sshDial(req.Addr, req.SSHPort, req.SSHUser, req.SSHPass)
	if err != nil {
		addStep("connect", "error", fmt.Sprintf("SSH connection failed: %v", err))
		return &ProvisionResult{Steps: steps}, fmt.Errorf("ssh dial: %w", err)
	}
	defer client.Close()
	addStep("connect", "done", fmt.Sprintf("Connected to %s:%d", req.Addr, req.SSHPort))

	// 2. Verify OpenWrt
	out, err := runCommand(client, "cat /etc/openwrt_release 2>/dev/null")
	if err != nil || !strings.Contains(out, "DISTRIB") {
		addStep("verify", "error", "Not an OpenWrt device")
		return &ProvisionResult{Steps: steps}, fmt.Errorf("not an OpenWrt device")
	}
	addStep("verify", "done", "OpenWrt verified")

	// 3. Install conntrack if needed (OpenWrt package is "conntrack", not "conntrack-tools")
	out, err = runCommand(client, "which conntrack >/dev/null 2>&1 && echo INSTALLED || echo MISSING")
	if err != nil {
		addStep("dependencies", "error", fmt.Sprintf("Failed to check packages: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}
	if strings.TrimSpace(out) == "MISSING" {
		out, err = runCommand(client, "opkg update && opkg install conntrack")
		if err != nil {
			addStep("dependencies", "error", fmt.Sprintf("Failed to install conntrack: %s", strings.TrimSpace(out)))
			return &ProvisionResult{Steps: steps}, fmt.Errorf("opkg install failed: %w", err)
		}
		addStep("dependencies", "done", "Installed conntrack")
	} else {
		addStep("dependencies", "done", "conntrack already available")
	}

	// 4. Generate API key and render config
	apiKey, err := db.GenerateAPIKey()
	if err != nil {
		addStep("deploy", "error", fmt.Sprintf("Failed to generate API key: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}

	lanPrefix := deriveLANPrefix(req.Addr, req.LANSubnet)

	agentCfg := RenderAgentConfig(AgentConfig{
		ServerURL:  req.ServerURL,
		APIKey:     apiKey,
		RouterName: req.Name,
		LANPrefix:  lanPrefix,
	})

	// 5. Deploy files
	if _, err := runCommand(client, "mkdir -p /etc/conntrack-agent"); err != nil {
		addStep("deploy", "error", fmt.Sprintf("Failed to create directory: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}

	if err := writeRemoteFile(client, "/etc/conntrack-agent/config", agentCfg); err != nil {
		addStep("deploy", "error", fmt.Sprintf("Failed to write config: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}

	if err := writeRemoteFile(client, "/etc/conntrack-agent/agent.sh", AgentScript()); err != nil {
		addStep("deploy", "error", fmt.Sprintf("Failed to write agent script: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}

	if err := writeRemoteFile(client, "/etc/init.d/conntrack-agent", InitdScript()); err != nil {
		addStep("deploy", "error", fmt.Sprintf("Failed to write init.d script: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}

	if _, err := runCommand(client, "chmod +x /etc/conntrack-agent/agent.sh /etc/init.d/conntrack-agent"); err != nil {
		addStep("deploy", "error", fmt.Sprintf("Failed to chmod: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}
	addStep("deploy", "done", "Agent files deployed")

	// 6. Enable and start service
	if _, err := runCommand(client, "/etc/init.d/conntrack-agent enable"); err != nil {
		addStep("start", "error", fmt.Sprintf("Failed to enable service: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}

	if _, err := runCommand(client, "/etc/init.d/conntrack-agent start"); err != nil {
		addStep("start", "error", fmt.Sprintf("Failed to start service: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}

	// Brief pause then verify
	time.Sleep(time.Second)
	out, err = runCommand(client, "pgrep -f 'conntrack-agent/agent.sh' >/dev/null 2>&1 && echo RUNNING || echo STOPPED")
	if err != nil || strings.TrimSpace(out) != "RUNNING" {
		addStep("start", "error", "Service started but process not found")
		return &ProvisionResult{Steps: steps}, fmt.Errorf("agent not running after start")
	}
	addStep("start", "done", "Service running")

	// 7. Store router record
	router := &db.Router{
		Name:      req.Name,
		Addr:      req.Addr,
		SSHPort:   req.SSHPort,
		SSHUser:   req.SSHUser,
		APIKey:    apiKey,
		LANSubnet: lanPrefix,
		Status:    "active",
	}
	if err := p.db.InsertRouter(router); err != nil {
		addStep("register", "error", fmt.Sprintf("Failed to save router: %v", err))
		return &ProvisionResult{Steps: steps}, err
	}
	addStep("register", "done", "Router registered")

	return &ProvisionResult{Router: router, Steps: steps}, nil
}

func (p *Provisioner) Deprovision(ctx context.Context, addr string, sshPort int, sshUser, sshPass string) ([]ProvisionStep, error) {
	var steps []ProvisionStep

	client, err := sshDial(addr, sshPort, sshUser, sshPass)
	if err != nil {
		steps = append(steps, ProvisionStep{"connect", "error", fmt.Sprintf("SSH failed: %v", err)})
		return steps, err
	}
	defer client.Close()
	steps = append(steps, ProvisionStep{"connect", "done", "Connected"})

	runCommand(client, "/etc/init.d/conntrack-agent stop")
	runCommand(client, "/etc/init.d/conntrack-agent disable")
	runCommand(client, "rm -rf /etc/conntrack-agent /etc/init.d/conntrack-agent")
	steps = append(steps, ProvisionStep{"remove", "done", "Agent removed"})

	return steps, nil
}

// --- helpers ---

func sshDial(addr string, port int, user, pass string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	return ssh.Dial("tcp", fmt.Sprintf("%s:%d", addr, port), config)
}

func runCommand(client *ssh.Client, cmd string) (string, error) {
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

func writeRemoteFile(client *ssh.Client, path, content string) error {
	// Base64-encode content to avoid any shell injection via heredoc delimiters
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	cmd := fmt.Sprintf("echo '%s' | base64 -d > %s", encoded, path)
	_, err := runCommand(client, cmd)
	return err
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
