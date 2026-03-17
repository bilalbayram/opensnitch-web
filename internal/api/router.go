package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gorilla/websocket"

	"github.com/evilsocket/opensnitch-web/internal/blocklist"
	"github.com/evilsocket/opensnitch-web/internal/config"
	"github.com/evilsocket/opensnitch-web/internal/db"
	"github.com/evilsocket/opensnitch-web/internal/nodemanager"
	"github.com/evilsocket/opensnitch-web/internal/prompter"
	"github.com/evilsocket/opensnitch-web/internal/templatesync"
	"github.com/evilsocket/opensnitch-web/internal/updater"
	"github.com/evilsocket/opensnitch-web/internal/ws"
)

type API struct {
	cfg          *config.Config
	db           *db.Database
	nodes        *nodemanager.Manager
	hub          *ws.Hub
	prompter     *prompter.Prompter
	templateSync *templatesync.Service
	fetcher      *blocklist.Fetcher
	updater      *updater.Updater
	upgrader     websocket.Upgrader
}

func NewRouter(cfg *config.Config, database *db.Database, nodes *nodemanager.Manager, hub *ws.Hub, p *prompter.Prompter, templateSync *templatesync.Service, frontendFS fs.FS, upd *updater.Updater) http.Handler {
	api := &API{
		cfg:          cfg,
		db:           database,
		nodes:        nodes,
		hub:          hub,
		prompter:     p,
		templateSync: templateSync,
		fetcher:      blocklist.NewFetcher(),
		updater:      upd,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}

	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(LoggingMiddleware)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	// Public routes
	r.Post("/api/v1/auth/login", api.handleLogin)
	r.Post("/api/v1/auth/logout", api.handleLogout)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(JWTAuthMiddleware(&cfg.Auth))

		r.Get("/api/v1/auth/me", api.handleMe)

		// WebSocket
		r.Get("/api/v1/ws", api.handleWebSocket)

		// Nodes
		r.Get("/api/v1/nodes", api.handleGetNodes)
		r.Get("/api/v1/nodes/{addr}", api.handleGetNode)
		r.Put("/api/v1/nodes/{addr}/config", api.handleUpdateNodeConfig)
		r.Put("/api/v1/nodes/{addr}/tags", api.handleReplaceNodeTags)
		r.Post("/api/v1/nodes/{addr}/interception/enable", api.handleNodeAction(true, false))
		r.Post("/api/v1/nodes/{addr}/interception/disable", api.handleNodeAction(false, false))
		r.Put("/api/v1/nodes/{addr}/mode", api.handleSetNodeMode)
		r.Post("/api/v1/nodes/{addr}/firewall/enable", api.handleNodeAction(true, true))
		r.Post("/api/v1/nodes/{addr}/firewall/disable", api.handleNodeAction(false, true))

		// Process Trust
		r.Get("/api/v1/nodes/{addr}/trust", api.handleGetProcessTrust)
		r.Post("/api/v1/nodes/{addr}/trust", api.handleAddProcessTrust)
		r.Put("/api/v1/nodes/{addr}/trust/{id}", api.handleUpdateProcessTrust)
		r.Delete("/api/v1/nodes/{addr}/trust/{id}", api.handleDeleteProcessTrust)

		// Rules
		r.Get("/api/v1/rules", api.handleGetRules)
		r.Post("/api/v1/rules", api.handleCreateRule)
		r.Post("/api/v1/rules/generate/preview", api.handleGenerateRulesPreview)
		r.Post("/api/v1/rules/generate/apply", api.handleGenerateRulesApply)
		r.Put("/api/v1/rules/{name}", api.handleUpdateRule)
		r.Delete("/api/v1/rules/{name}", api.handleDeleteRule)
		r.Post("/api/v1/rules/{name}/enable", api.handleToggleRule(true))
		r.Post("/api/v1/rules/{name}/disable", api.handleToggleRule(false))

		// Templates
		r.Get("/api/v1/templates", api.handleGetTemplates)
		r.Post("/api/v1/templates", api.handleCreateTemplate)
		r.Get("/api/v1/templates/{id}", api.handleGetTemplate)
		r.Put("/api/v1/templates/{id}", api.handleUpdateTemplate)
		r.Delete("/api/v1/templates/{id}", api.handleDeleteTemplate)
		r.Post("/api/v1/templates/{id}/rules", api.handleCreateTemplateRule)
		r.Put("/api/v1/templates/{id}/rules/{ruleId}", api.handleUpdateTemplateRule)
		r.Delete("/api/v1/templates/{id}/rules/{ruleId}", api.handleDeleteTemplateRule)
		r.Post("/api/v1/templates/{id}/attachments", api.handleCreateTemplateAttachment)
		r.Put("/api/v1/templates/{id}/attachments/{attachmentId}", api.handleUpdateTemplateAttachment)
		r.Delete("/api/v1/templates/{id}/attachments/{attachmentId}", api.handleDeleteTemplateAttachment)

		// Connections
		r.Get("/api/v1/connections", api.handleGetConnections)
		r.Delete("/api/v1/connections", api.handlePurgeConnections)
		r.Get("/api/v1/seen-flows", api.handleGetSeenFlows)

		// DNS
		r.Get("/api/v1/dns/domains", api.handleGetDNSDomains)
		r.Delete("/api/v1/dns/domains", api.handlePurgeDNSDomains)
		r.Get("/api/v1/dns/servers", api.handleGetDNSServers)
		r.Post("/api/v1/dns/server-rules", api.handleCreateDNSServerRules)

		// Stats
		r.Get("/api/v1/stats", api.handleGetGeneralStats)
		r.Get("/api/v1/stats/{table}", api.handleGetStats)

		// Firewall
		r.Get("/api/v1/firewall", api.handleGetFirewall)
		r.Post("/api/v1/firewall/reload", api.handleReloadFirewall)

		// Alerts
		r.Get("/api/v1/alerts", api.handleGetAlerts)
		r.Delete("/api/v1/alerts/{id}", api.handleDeleteAlert)

		// Blocklists
		r.Get("/api/v1/blocklists", api.handleGetBlocklists)
		r.Post("/api/v1/blocklists", api.handleCreateBlocklist)
		r.Delete("/api/v1/blocklists/{id}", api.handleDeleteBlocklist)
		r.Post("/api/v1/blocklists/{id}/enable", api.handleEnableBlocklist)
		r.Post("/api/v1/blocklists/{id}/disable", api.handleDisableBlocklist)
		r.Post("/api/v1/blocklists/{id}/sync", api.handleSyncBlocklist)

		// Prompts
		r.Get("/api/v1/prompts/pending", api.handleGetPendingPrompts)
		r.Post("/api/v1/prompts/{id}/reply", api.handlePromptReply)

		// Version & Updates
		r.Get("/api/v1/version", api.handleGetVersion)
		r.Post("/api/v1/update/check", api.handleCheckUpdate)
		r.Post("/api/v1/update/apply", api.handleApplyUpdate)
	})

	// Serve frontend — SPA fallback: serve index.html for non-API, non-file routes
	if frontendFS != nil {
		fileServer := http.FileServerFS(frontendFS)
		r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/")
			// Try to serve the file directly
			if path != "" {
				if _, err := fs.Stat(frontendFS, path); err == nil {
					fileServer.ServeHTTP(w, r)
					return
				}
			}
			// SPA fallback: serve index.html
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
		})
	}

	return r
}
