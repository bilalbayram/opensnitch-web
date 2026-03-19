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

	if req.Addr == "" || (req.SSHPass == "" && req.SSHKey == "") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "addr and ssh_pass (or ssh_key) are required"})
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

	// Resolve server URL: user override > LAN auto-detect > Host header fallback
	var serverURLSource string
	if req.ServerURL != "" {
		serverURLSource = "user_override"
	} else if lanURL, source := router.ResolveServerURL(req.Addr, a.cfg.Server.HTTPAddr); lanURL != "" {
		req.ServerURL = lanURL
		serverURLSource = source
	} else {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		req.ServerURL = fmt.Sprintf("%s://%s", scheme, r.Host)
		serverURLSource = "host_fallback"
		log.Printf("[router] No shared subnet with %s (reason: %s), falling back to Host header: %s", req.Addr, source, req.ServerURL)
	}

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

	result.ServerURL = req.ServerURL
	result.ServerURLSource = serverURLSource

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
			online = routerOnlineFromLastConn(node.LastConn)
		}
		result[i] = enrichedRouter{Router: rt, Online: online}
	}

	writeJSON(w, http.StatusOK, result)
}

func (a *API) handleScanRouters(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Subnet string `json:"subnet"`
	}
	// Subnet is optional — auto-detect if not provided
	readJSON(r, &req)

	subnet := req.Subnet
	if subnet == "" {
		detected, err := router.DetectLocalSubnet()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not detect local subnet: " + err.Error()})
			return
		}
		subnet = detected
	}

	results, err := router.ScanSubnet(subnet)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if results == nil {
		results = []router.DiscoveredRouter{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"subnet":  subnet,
		"devices": results,
	})
}

func (a *API) handleDisconnectRouter(w http.ResponseWriter, r *http.Request) {
	addr := chi.URLParam(r, "addr")
	// URL-decode: chi may pass encoded colons etc.
	if decoded, err := url.QueryUnescape(addr); err == nil {
		addr = decoded
	}

	var req struct {
		SSHPass string `json:"ssh_pass"`
		SSHKey  string `json:"ssh_key"`
	}
	if err := readJSON(r, &req); err != nil || (req.SSHPass == "" && req.SSHKey == "") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ssh_pass or ssh_key is required"})
		return
	}

	rt, err := a.db.GetRouterByAddr(addr)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "router not found"})
		return
	}

	steps, deprovErr := a.routerProv.Deprovision(r.Context(), rt.Addr, rt.SSHPort, rt.SSHUser, req.SSHPass, req.SSHKey)
	if deprovErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": deprovErr.Error(),
			"steps": steps,
		})
		return
	}

	if err := a.db.DeleteRouter(rt.Addr); err != nil {
		log.Printf("[router] Failed to delete router record %s: %v", rt.Addr, err)
	}
	a.db.SetNodeStatus(rt.Addr, "offline")

	a.hub.BroadcastEvent(ws.EventNodeDisconnected, map[string]any{
		"addr": rt.Addr,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"steps":  steps,
	})
}

func (a *API) handleSuggestServerURL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RouterIP string `json:"router_ip"`
	}
	if err := readJSON(r, &req); err != nil || req.RouterIP == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "router_ip is required"})
		return
	}

	serverURL, source := router.ResolveServerURL(req.RouterIP, a.cfg.Server.HTTPAddr)

	resp := map[string]string{
		"server_url": serverURL,
		"source":     source,
	}
	if serverURL == "" {
		resp["warning"] = "Server and router do not appear to share a subnet. The router may not be able to reach the server. You can enter a server URL manually."
	}
	writeJSON(w, http.StatusOK, resp)
}
