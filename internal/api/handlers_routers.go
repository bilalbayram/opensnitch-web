package api

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/evilsocket/opensnitch-web/internal/db"
	"github.com/evilsocket/opensnitch-web/internal/router"
	"github.com/evilsocket/opensnitch-web/internal/ws"
)

func (a *API) handleConnectRouter(w http.ResponseWriter, r *http.Request) {
	var req router.ConnectRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Addr == "" || req.SSHPass == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "addr and ssh_pass are required"})
		return
	}

	if req.SSHPort == 0 {
		req.SSHPort = 22
	}
	if req.SSHUser == "" {
		req.SSHUser = "root"
	}
	if req.Name == "" {
		req.Name = req.Addr
	}

	// Auto-detect server URL from the request
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	req.ServerURL = fmt.Sprintf("%s://%s", scheme, r.Host)

	// Auto-detect LAN subnet from router IP if not provided
	if req.LANSubnet == "" {
		parts := strings.Split(req.Addr, ".")
		if len(parts) == 4 {
			req.LANSubnet = fmt.Sprintf("%s.%s.%s.0/24", parts[0], parts[1], parts[2])
		}
	}

	result, err := a.routerProv.Provision(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": err.Error(),
			"steps": result.Steps,
		})
		return
	}

	// Register as a node
	a.db.UpsertNode(&db.Node{
		Addr:          result.Router.Addr,
		Hostname:      result.Router.Name,
		DaemonVersion: "conntrack-agent",
		Status:        "online",
		LastConn:      time.Now().Format("2006-01-02 15:04:05"),
		SourceType:    "router",
	})

	a.hub.BroadcastEvent(ws.EventNodeConnected, map[string]any{
		"addr":        result.Router.Addr,
		"hostname":    result.Router.Name,
		"version":     "conntrack-agent",
		"source_type": "router",
	})

	writeJSON(w, http.StatusOK, result)
}

func (a *API) handleGetRouters(w http.ResponseWriter, r *http.Request) {
	routers, err := a.db.GetRouters()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if routers == nil {
		routers = []db.Router{}
	}

	// Enrich with node online status
	type enrichedRouter struct {
		db.Router
		Online bool `json:"online"`
	}

	result := make([]enrichedRouter, len(routers))
	for i, rt := range routers {
		node, err := a.db.GetNode(rt.Addr)
		online := false
		if err == nil && node != nil {
			// Consider online if last connection was within 60 seconds
			if t, err := time.Parse("2006-01-02 15:04:05", node.LastConn); err == nil {
				online = time.Since(t) < 60*time.Second
			}
		}
		result[i] = enrichedRouter{Router: rt, Online: online}
	}

	writeJSON(w, http.StatusOK, result)
}

func (a *API) handleDisconnectRouter(w http.ResponseWriter, r *http.Request) {
	addr := chi.URLParam(r, "addr")
	// URL-decode: chi may pass encoded colons etc.
	if decoded, err := url.QueryUnescape(addr); err == nil {
		addr = decoded
	}

	var req struct {
		SSHPass string `json:"ssh_pass"`
	}
	if err := readJSON(r, &req); err != nil || req.SSHPass == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ssh_pass is required"})
		return
	}

	rt, err := a.db.GetRouterByAddr(addr)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "router not found"})
		return
	}

	steps, deprovErr := a.routerProv.Deprovision(r.Context(), rt.Addr, rt.SSHPort, rt.SSHUser, req.SSHPass)

	// Clean up DB even if deprovision had issues
	if err := a.db.DeleteRouter(rt.Addr); err != nil {
		log.Printf("[router] Failed to delete router record %s: %v", rt.Addr, err)
	}
	a.db.SetNodeStatus(rt.Addr, "offline")

	a.hub.BroadcastEvent(ws.EventNodeDisconnected, map[string]any{
		"addr": rt.Addr,
	})

	if deprovErr != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "removed_with_errors",
			"steps":   steps,
			"warning": deprovErr.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"steps":  steps,
	})
}
