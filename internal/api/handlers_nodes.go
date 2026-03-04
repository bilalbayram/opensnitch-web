package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	pb "github.com/evilsocket/opensnitch-web/proto"
)

func (a *API) handleGetNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := a.db.GetNodes()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Enrich with live status from node manager
	liveNodes := a.nodes.GetAllNodes()
	type enrichedNode struct {
		Addr          string `json:"addr"`
		Hostname      string `json:"hostname"`
		DaemonVersion string `json:"daemon_version"`
		DaemonUptime  int64  `json:"daemon_uptime"`
		DaemonRules   int64  `json:"daemon_rules"`
		Cons          int64  `json:"cons"`
		ConsDropped   int64  `json:"cons_dropped"`
		Status        string `json:"status"`
		LastConn      string `json:"last_connection"`
		Online        bool   `json:"online"`
		Mode          string `json:"mode"`
	}

	result := make([]enrichedNode, len(nodes))
	for i, n := range nodes {
		_, online := liveNodes[n.Addr]
		status := n.Status
		if online {
			status = "online"
		}
		result[i] = enrichedNode{
			Addr:          n.Addr,
			Hostname:      n.Hostname,
			DaemonVersion: n.DaemonVersion,
			DaemonUptime:  n.DaemonUptime,
			DaemonRules:   n.DaemonRules,
			Cons:          n.Cons,
			ConsDropped:   n.ConsDropped,
			Status:        status,
			LastConn:      n.LastConn,
			Online:        online,
			Mode:          n.Mode,
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
	writeJSON(w, http.StatusOK, node)
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
