package api

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/bilalbayram/opensnitch-web/internal/db"
	"github.com/bilalbayram/opensnitch-web/internal/router"
	"github.com/bilalbayram/opensnitch-web/internal/ws"
)

type routerSSHRequest struct {
	SSHPass string `json:"ssh_pass"`
	SSHKey  string `json:"ssh_key"`
}

type routerUpgradeRequest struct {
	SSHPass         string `json:"ssh_pass"`
	SSHKey          string `json:"ssh_key"`
	NodeName        string `json:"node_name"`
	DefaultAction   string `json:"default_action"`
	PollIntervalMS  int    `json:"poll_interval_ms"`
	FirewallBackend string `json:"firewall_backend"`
}

type routerDowngradeRequest struct {
	SSHPass   string `json:"ssh_pass"`
	SSHKey    string `json:"ssh_key"`
	ServerURL string `json:"server_url"`
}

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

	connectMode := router.NormalizeConnectMode(req.Mode)
	if connectMode == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mode must be monitor or manage"})
		return
	}
	req.Mode = connectMode

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

	if connectMode == router.ConnectModeManage {
		daemonResult, daemonErr := a.routerProv.ProvisionDaemon(r.Context(), router.DaemonRequest{
			Addr:     req.Addr,
			SSHPort:  req.SSHPort,
			SSHUser:  req.SSHUser,
			SSHPass:  req.SSHPass,
			SSHKey:   req.SSHKey,
			NodeName: req.Name,
		})
		if daemonResult != nil {
			result.Steps = append(result.Steps, daemonResult.Steps...)
			if daemonResult.Router != nil {
				result.Router = daemonResult.Router
			}
			if daemonResult.Capabilities != nil {
				result.Capabilities = daemonResult.Capabilities
			}
		}
		if daemonErr != nil {
			result.Warning = fmt.Sprintf("Router connected in monitor mode. Manage setup failed: %v", daemonErr)
		}
	}

	legacyConnected := result.Router != nil && result.Router.DaemonMode != db.RouterDaemonModeRouterDaemon
	if legacyConnected {
		// Register as a node (UpsertRouterNode preserves existing cons counter)
		a.db.UpsertRouterNode(result.Router.Addr, result.Router.Name, "conntrack-agent", db.NodeStatusOnline, time.Now().Format("2006-01-02 15:04:05"))

		a.hub.BroadcastEvent(ws.EventNodeConnected, map[string]any{
			"addr":        result.Router.Addr,
			"hostname":    result.Router.Name,
			"version":     "conntrack-agent",
			"source_type": "router",
		})
	}

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
	type linkedNodeSummary struct {
		Addr           string `json:"addr"`
		Online         bool   `json:"online"`
		Mode           string `json:"mode"`
		DaemonVersion  string `json:"daemon_version"`
		DaemonRules    int64  `json:"daemon_rules"`
		Cons           int64  `json:"cons"`
		ConsDropped    int64  `json:"cons_dropped"`
		LastConnection string `json:"last_connection"`
	}
	type enrichedRouter struct {
		db.Router
		Online     bool               `json:"online"`
		LinkedNode *linkedNodeSummary `json:"linked_node"`
	}

	result := make([]enrichedRouter, len(routers))
	for i, rt := range routers {
		nodeAddr := rt.Addr
		if rt.DaemonMode == db.RouterDaemonModeRouterDaemon && strings.TrimSpace(rt.LinkedNodeAddr) != "" {
			nodeAddr = rt.LinkedNodeAddr
		}

		node, err := a.db.GetNode(nodeAddr)
		online := false
		var linkedNode *linkedNodeSummary
		if err == nil && node != nil {
			online = routerOnlineFromLastConn(node.LastConn)
			linkedNode = &linkedNodeSummary{
				Addr:           nodeAddr,
				Online:         online,
				Mode:           node.Mode,
				DaemonVersion:  node.DaemonVersion,
				DaemonRules:    node.DaemonRules,
				Cons:           node.Cons,
				ConsDropped:    node.ConsDropped,
				LastConnection: node.LastConn,
			}
		}
		result[i] = enrichedRouter{Router: rt, Online: online, LinkedNode: linkedNode}
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
	a.db.SetNodeStatus(rt.Addr, db.NodeStatusOffline)

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

func (a *API) handleRouterCapabilities(w http.ResponseWriter, r *http.Request) {
	addr := routerAddrParam(r)

	var req routerSSHRequest
	if err := readJSON(r, &req); err != nil || (req.SSHPass == "" && req.SSHKey == "") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ssh_pass or ssh_key is required"})
		return
	}

	rt, err := a.db.GetRouterByAddr(addr)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "router not found"})
		return
	}

	result, err := a.routerProv.CheckCapabilities(r.Context(), rt.Addr, rt.SSHPort, rt.SSHUser, req.SSHPass, req.SSHKey)
	if err != nil {
		var capabilities any
		var steps any
		if result != nil {
			capabilities = result.Capabilities
			steps = result.Steps
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"capabilities": capabilities,
			"steps":        steps,
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (a *API) handleUpgradeRouter(w http.ResponseWriter, r *http.Request) {
	addr := routerAddrParam(r)

	var req routerUpgradeRequest
	if err := readJSON(r, &req); err != nil || (req.SSHPass == "" && req.SSHKey == "") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ssh_pass or ssh_key is required"})
		return
	}

	rt, err := a.db.GetRouterByAddr(addr)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "router not found"})
		return
	}
	if rt.DaemonMode == db.RouterDaemonModeRouterDaemon {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "router is already running router-daemon"})
		return
	}

	result, err := a.routerProv.ProvisionDaemon(r.Context(), router.DaemonRequest{
		Addr:            rt.Addr,
		SSHPort:         rt.SSHPort,
		SSHUser:         rt.SSHUser,
		SSHPass:         req.SSHPass,
		SSHKey:          req.SSHKey,
		NodeName:        req.NodeName,
		DefaultAction:   req.DefaultAction,
		PollIntervalMS:  req.PollIntervalMS,
		FirewallBackend: req.FirewallBackend,
	})
	if err != nil {
		var capabilities any
		var steps any
		if result != nil {
			capabilities = result.Capabilities
			steps = result.Steps
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":        err.Error(),
			"capabilities": capabilities,
			"steps":        steps,
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (a *API) handleDowngradeRouter(w http.ResponseWriter, r *http.Request) {
	addr := routerAddrParam(r)

	var req routerDowngradeRequest
	if err := readJSON(r, &req); err != nil || (req.SSHPass == "" && req.SSHKey == "") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ssh_pass or ssh_key is required"})
		return
	}

	rt, err := a.db.GetRouterByAddr(addr)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "router not found"})
		return
	}
	if rt.DaemonMode != db.RouterDaemonModeRouterDaemon {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "router is not running router-daemon"})
		return
	}

	deprovisionSteps, err := a.routerProv.DeprovisionDaemon(r.Context(), rt.Addr, rt.SSHPort, rt.SSHUser, req.SSHPass, req.SSHKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": err.Error(),
			"steps": deprovisionSteps,
		})
		return
	}

	serverURL := strings.TrimSpace(req.ServerURL)
	if serverURL == "" {
		if lanURL, _ := router.ResolveServerURL(rt.Addr, a.cfg.Server.HTTPAddr); lanURL != "" {
			serverURL = lanURL
		} else {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			serverURL = fmt.Sprintf("%s://%s", scheme, r.Host)
		}
	}

	result, provisionErr := a.routerProv.Provision(r.Context(), router.ConnectRequest{
		Addr:      rt.Addr,
		SSHPort:   rt.SSHPort,
		SSHUser:   rt.SSHUser,
		SSHPass:   req.SSHPass,
		SSHKey:    req.SSHKey,
		Name:      rt.Name,
		LANSubnet: rt.LANSubnet,
		ServerURL: serverURL,
	})
	if provisionErr != nil {
		steps := append([]router.ProvisionStep{}, deprovisionSteps...)
		if result != nil {
			steps = append(steps, result.Steps...)
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": provisionErr.Error(),
			"steps": steps,
		})
		return
	}

	if err := a.db.UpdateRouterRuntime(rt.Addr, db.RouterDaemonModeConntrackAgent, ""); err != nil {
		log.Printf("[router] Failed to update router runtime for %s: %v", rt.Addr, err)
	}
	_ = a.db.UpsertRouterNode(rt.Addr, rt.Name, "conntrack-agent", db.NodeStatusOnline, time.Now().Format("2006-01-02 15:04:05"))

	result.Steps = append(deprovisionSteps, result.Steps...)
	writeJSON(w, http.StatusOK, result)
}

func routerAddrParam(r *http.Request) string {
	addr := chi.URLParam(r, "addr")
	if decoded, err := url.QueryUnescape(addr); err == nil {
		return decoded
	}
	return addr
}
