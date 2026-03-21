package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bilalbayram/opensnitch-web/internal/router"
	"github.com/bilalbayram/opensnitch-web/internal/version"
	pb "github.com/bilalbayram/opensnitch-web/proto"
	"google.golang.org/grpc"
)

const defaultConfigPath = "/etc/opensnitchd-router/config.json"

// opensnitchd-router is the managed OpenWrt router runtime for v1.
// It only runs live AskRule prompts for router-local processes. Forwarded LAN
// flows are observed continuously and only enforced when an explicit device
// rule already exists.
type daemon struct {
	cfg        router.DaemonConfig
	configJSON string
	logger     *log.Logger

	pollInterval  time.Duration
	rulesPath     string
	defaultAction string

	conn       *grpc.ClientConn
	client     pb.UIClient
	notif      pb.UI_NotificationsClient
	notifClose sync.Once

	stateMu             sync.RWMutex
	interceptionEnabled bool
	firewallEnabled     bool

	rulesMu sync.RWMutex
	rules   map[string]*ruleEntry

	stats *statsCollector
}

func main() {
	configPath := flag.String("config", defaultConfigPath, "Path to router-daemon config")
	flag.Parse()

	cfg, rawConfig, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("[router-daemon] load config: %v", err)
	}

	logger := log.New(os.Stdout, "[router-daemon] ", log.LstdFlags|log.Lmsgprefix)
	d := &daemon{
		cfg:                 cfg,
		configJSON:          rawConfig,
		logger:              logger,
		pollInterval:        time.Duration(cfg.PollIntervalMS) * time.Millisecond,
		rulesPath:           filepath.Join(filepath.Dir(*configPath), "rules.json"),
		defaultAction:       strings.ToLower(strings.TrimSpace(cfg.DefaultAction)),
		interceptionEnabled: true,
		firewallEnabled:     true,
		rules:               make(map[string]*ruleEntry),
		stats:               newStatsCollector(),
	}

	if err := d.loadRules(); err != nil {
		logger.Fatalf("load cached rules: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := d.run(ctx); err != nil && ctx.Err() == nil {
		logger.Fatalf("runtime failed: %v", err)
	}
}

func loadConfig(path string) (router.DaemonConfig, string, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return router.DaemonConfig{}, "", err
	}

	var cfg router.DaemonConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return router.DaemonConfig{}, "", err
	}

	if strings.TrimSpace(cfg.GRPCAddr) == "" {
		return router.DaemonConfig{}, "", fmt.Errorf("grpc_addr is required")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return router.DaemonConfig{}, "", fmt.Errorf("api_key is required")
	}
	if strings.TrimSpace(cfg.NodeName) == "" {
		return router.DaemonConfig{}, "", fmt.Errorf("node_name is required")
	}
	if strings.TrimSpace(cfg.DefaultAction) == "" {
		cfg.DefaultAction = "deny"
	}
	if cfg.PollIntervalMS <= 0 {
		cfg.PollIntervalMS = 1000
	}
	if strings.TrimSpace(cfg.FirewallBackend) == "" {
		cfg.FirewallBackend = "nft"
	}
	if strings.TrimSpace(cfg.FirewallBackend) != "nft" {
		return router.DaemonConfig{}, "", fmt.Errorf("firewall_backend %q is not supported in v1", cfg.FirewallBackend)
	}

	return cfg, string(data), nil
}

func (d *daemon) run(ctx context.Context) error {
	if err := d.connect(ctx); err != nil {
		return err
	}
	defer d.close()

	if err := d.ensureFirewall(); err != nil {
		return err
	}
	if err := d.rebuildForwardRules(); err != nil {
		return err
	}

	errCh := make(chan error, 2)
	go func() { errCh <- d.runPingLoop(ctx) }()
	go func() { errCh <- d.runNotifications(ctx) }()
	go d.runLocalMonitor(ctx)
	go d.runForwardMonitor(ctx)

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func (d *daemon) close() {
	d.notifClose.Do(func() {
		if d.notif != nil {
			_ = d.notif.CloseSend()
		}
	})
	if d.conn != nil {
		_ = d.conn.Close()
	}
}

func daemonVersion() string {
	if strings.TrimSpace(version.Version) == "" {
		return "opensnitchd-router"
	}
	return "opensnitchd-router/" + version.Version
}
