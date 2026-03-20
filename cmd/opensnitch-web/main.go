package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/bilalbayram/opensnitch-web/internal/api"
	"github.com/bilalbayram/opensnitch-web/internal/auth"
	"github.com/bilalbayram/opensnitch-web/internal/config"
	"github.com/bilalbayram/opensnitch-web/internal/db"
	"github.com/bilalbayram/opensnitch-web/internal/geoip"
	"github.com/bilalbayram/opensnitch-web/internal/grpcserver"
	"github.com/bilalbayram/opensnitch-web/internal/nodemanager"
	"github.com/bilalbayram/opensnitch-web/internal/prompter"
	"github.com/bilalbayram/opensnitch-web/internal/templatesync"
	"github.com/bilalbayram/opensnitch-web/internal/version"
	"github.com/bilalbayram/opensnitch-web/internal/ws"
)

//go:embed all:frontend
var embeddedFrontend embed.FS

func main() {
	configPath := flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	database, err := db.New(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Create default admin user
	if err := auth.EnsureDefaultUser(database, &cfg.Auth); err != nil {
		log.Fatalf("Failed to ensure default user: %v", err)
	}

	// Initialize components
	nodes := nodemanager.NewManager()
	hub := ws.NewHub()
	p := prompter.New(cfg.UI.PromptTimeout)
	templateSync := templatesync.New(database, nodes)

	// Wire up prompter → WebSocket broadcasts
	p.OnNewPrompt = func(prompt *prompter.PendingPrompt) {
		conn := prompt.Connection
		hub.BroadcastEvent(ws.EventPromptRequest, map[string]interface{}{
			"id":         prompt.ID,
			"node_addr":  prompt.NodeAddr,
			"created_at": prompt.CreatedAt.Format("2006-01-02 15:04:05"),
			"process":    conn.GetProcessPath(),
			"dst_host":   conn.GetDstHost(),
			"dst_ip":     conn.GetDstIp(),
			"dst_port":   conn.GetDstPort(),
			"protocol":   conn.GetProtocol(),
			"src_ip":     conn.GetSrcIp(),
			"src_port":   conn.GetSrcPort(),
			"uid":        conn.GetUserId(),
			"pid":        conn.GetProcessId(),
			"args":       conn.GetProcessArgs(),
			"cwd":        conn.GetProcessCwd(),
			"checksums":  conn.GetProcessChecksums(),
		})
	}

	p.OnPromptTimeout = func(promptID string) {
		hub.BroadcastEvent(ws.EventPromptTimeout, map[string]interface{}{
			"id": promptID,
		})
	}

	nodes.OnNodeDisconnected = func(addr string) {
		database.SetNodeStatus(addr, db.NodeStatusOffline)
		hub.BroadcastEvent(ws.EventNodeDisconnected, map[string]interface{}{
			"addr": addr,
		})
	}

	// Print version
	log.Printf("OpenSnitch Web UI %s (built %s)", version.Version, version.BuildTime)

	// Signal channel for graceful shutdown.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start WebSocket hub
	go hub.Run()

	// Create gRPC server
	uiService := grpcserver.NewUIService(nodes, database, hub, p, templateSync)
	grpcSrv := grpcserver.New(uiService)

	go func() {
		if err := grpcSrv.ListenAndServe(cfg.Server.GRPCAddr); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// Also listen on unix socket for local daemon connections
	if cfg.Server.GRPCUnix != "" {
		os.Remove(cfg.Server.GRPCUnix) // clean up stale socket
		go func() {
			if err := grpcSrv.ListenUnix(cfg.Server.GRPCUnix); err != nil {
				log.Printf("[grpc] Unix socket failed: %v", err)
			}
		}()
	}

	// Resolve frontend FS: prefer local web/dist, fallback to embedded
	var frontendFS fs.FS
	if info, err := os.Stat("web/dist"); err == nil && info.IsDir() {
		log.Println("[http] Serving frontend from local web/dist/")
		frontendFS = os.DirFS("web/dist")
	} else {
		sub, err := fs.Sub(embeddedFrontend, "frontend")
		if err != nil {
			log.Printf("[http] No embedded frontend: %v", err)
		} else {
			log.Println("[http] Serving embedded frontend")
			frontendFS = sub
		}
	}

	// Create GeoIP resolver
	geo := geoip.NewResolver(database, cfg.GeoIP.Enabled)

	// Create HTTP server
	router := api.NewRouter(cfg, database, nodes, hub, p, templateSync, frontendFS, geo)
	httpSrv := &http.Server{Addr: cfg.Server.HTTPAddr, Handler: router}

	go func() {
		log.Printf("[http] Listening on %s", cfg.Server.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	log.Println("OpenSnitch Web UI started")
	log.Printf("  Web:   http://localhost%s", cfg.Server.HTTPAddr)
	log.Printf("  gRPC:  %s", cfg.Server.GRPCAddr)
	if cfg.Server.GRPCUnix != "" {
		log.Printf("  gRPC:  unix:%s", cfg.Server.GRPCUnix)
	}
	log.Println("  Login: admin / opensnitch")

	// Wait for shutdown
	<-sigChan

	log.Println("Shutting down...")
	geo.Stop()
	httpSrv.Shutdown(context.Background())
	grpcSrv.Stop()
}
