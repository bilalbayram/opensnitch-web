package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	pb "github.com/evilsocket/opensnitch-web/proto"
)

const routerHeartbeatTTL = 60 * time.Second

func routerOnlineFromLastConn(lastConn string) bool {
	if lastConn == "" {
		return false
	}

	t, err := time.ParseInLocation("2006-01-02 15:04:05", lastConn, time.Local)
	if err != nil {
		return false
	}

	return time.Since(t) < routerHeartbeatTTL
}

func (a *API) handleGetNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := a.db.GetNodes()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	nodeTags, err := a.db.GetAllNodeTags()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	syncStates, err := a.db.GetAllNodeTemplateSync()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Enrich with live status from node manager
	liveNodes := a.nodes.GetAllNodes()
	type enrichedNode struct {
		Addr                string   `json:"addr"`
		Hostname            string   `json:"hostname"`
		DaemonVersion       string   `json:"daemon_version"`
		DaemonUptime        int64    `json:"daemon_uptime"`
		DaemonRules         int64    `json:"daemon_rules"`
		Cons                int64    `json:"cons"`
		ConsDropped         int64    `json:"cons_dropped"`
		Status              string   `json:"status"`
		LastConn            string   `json:"last_connection"`
		Online              bool     `json:"online"`
		Mode                string   `json:"mode"`
		SourceType          string   `json:"source_type"`
		Tags                []string `json:"tags"`
		TemplateSyncPending bool     `json:"template_sync_pending"`
		TemplateSyncError   string   `json:"template_sync_error"`
	}

	result := make([]enrichedNode, len(nodes))
	for i, n := range nodes {
		sourceType := n.SourceType
		if sourceType == "" {
			sourceType = "opensnitch"
		}
		_, online := liveNodes[n.Addr]
		if sourceType == "router" {
			online = routerOnlineFromLastConn(n.LastConn)
		}

		status := n.Status
		if online {
			status = "online"
		} else if sourceType == "router" {
			status = "offline"
		}

		tags := nodeTags[n.Addr]
		if tags == nil {
			tags = []string{}
		}
		syncState := syncStates[n.Addr]
		result[i] = enrichedNode{
			Addr:                n.Addr,
			Hostname:            n.Hostname,
			DaemonVersion:       n.DaemonVersion,
			DaemonUptime:        n.DaemonUptime,
			DaemonRules:         n.DaemonRules,
			Cons:                n.Cons,
			ConsDropped:         n.ConsDropped,
			Status:              status,
			LastConn:            n.LastConn,
			Online:              online,
			Mode:                n.Mode,
			SourceType:          sourceType,
			Tags:                tags,
			TemplateSyncPending: syncState.Pending,
			TemplateSyncError:   syncState.Error,
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (a *API) handleGetNode(w http.ResponseWriter, r *http.Request) {
	addr := chi.URLParam(r, "addr")
	node, err := a.db.GetNode(addr)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}
	tags, err := a.db.GetNodeTags(addr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if tags == nil {
		tags = []string{}
	}
	syncState, err := a.db.GetNodeTemplateSync(addr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	sourceType := node.SourceType
	if sourceType == "" {
		sourceType = "opensnitch"
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"addr":                  node.Addr,
		"hostname":              node.Hostname,
		"daemon_version":        node.DaemonVersion,
		"daemon_uptime":         node.DaemonUptime,
		"daemon_rules":          node.DaemonRules,
		"cons":                  node.Cons,
		"cons_dropped":          node.ConsDropped,
		"version":               node.Version,
		"status":                node.Status,
		"last_connection":       node.LastConn,
		"mode":                  node.Mode,
		"source_type":           sourceType,
		"tags":                  tags,
		"template_sync_pending": syncState.Pending,
		"template_sync_error":   syncState.Error,
	})
}

func (a *API) handleReplaceNodeTags(w http.ResponseWriter, r *http.Request) {
	addr := chi.URLParam(r, "addr")
	if _, err := a.db.GetNode(addr); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}

	var req struct {
		Tags []string `json:"tags"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	tags, err := a.db.ReplaceNodeTags(addr, req.Tags)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if a.templateSync != nil {
		if err := a.templateSync.ReconcileNode(addr); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	syncState, err := a.db.GetNodeTemplateSync(addr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tags":                  tags,
		"template_sync_pending": syncState.Pending,
		"template_sync_error":   syncState.Error,
	})
}

func (a *API) handleUpdateNodeConfig(w http.ResponseWriter, r *http.Request) {
	addr := chi.URLParam(r, "addr")
	var configData map[string]interface{}
	if err := readJSON(r, &configData); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	configJSON, _ := json.Marshal(configData)

	notif := &pb.Notification{
		Id:   a.nodes.NextID(),
		Type: pb.Action_CHANGE_CONFIG,
		Data: string(configJSON),
	}

	if !a.nodes.SendNotification(addr, notif) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not connected"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) handleSetNodeMode(w http.ResponseWriter, r *http.Request) {
	addr := chi.URLParam(r, "addr")
	var req struct {
		Mode string `json:"mode"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	switch req.Mode {
	case "ask", "silent_allow", "silent_deny":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mode must be ask, silent_allow, or silent_deny"})
		return
	}

	if err := a.db.SetNodeMode(addr, req.Mode); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) handleNodeAction(enable bool, isFirewall bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addr := chi.URLParam(r, "addr")

		var actionType pb.Action
		if isFirewall {
			if enable {
				actionType = pb.Action_ENABLE_FIREWALL
			} else {
				actionType = pb.Action_DISABLE_FIREWALL
			}
		} else {
			if enable {
				actionType = pb.Action_ENABLE_INTERCEPTION
			} else {
				actionType = pb.Action_DISABLE_INTERCEPTION
			}
		}

		notif := &pb.Notification{
			Id:   a.nodes.NextID(),
			Type: actionType,
		}

		if !a.nodes.SendNotification(addr, notif) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not connected"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
