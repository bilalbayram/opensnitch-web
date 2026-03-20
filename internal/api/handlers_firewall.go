package api

import (
	"encoding/json"
	"net/http"

	pb "github.com/bilalbayram/opensnitch-web/proto"
)

func (a *API) handleGetFirewall(w http.ResponseWriter, r *http.Request) {
	nodes := a.nodes.GetAllNodes()

	type fwState struct {
		NodeAddr string          `json:"node_addr"`
		Running  bool            `json:"running"`
		Firewall *pb.SysFirewall `json:"firewall,omitempty"`
	}

	var result []fwState
	for addr, node := range nodes {
		result = append(result, fwState{
			NodeAddr: addr,
			Running:  node.IsFirewallRunning,
			Firewall: node.SystemFirewall,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (a *API) handleReloadFirewall(w http.ResponseWriter, r *http.Request) {
	addr := r.URL.Query().Get("node")

	// Optionally accept firewall rules in body
	var fwData json.RawMessage
	readJSON(r, &fwData)

	notif := &pb.Notification{
		Id:   a.nodes.NextID(),
		Type: pb.Action_RELOAD_FW_RULES,
	}

	if addr != "" {
		if !a.nodes.SendNotification(addr, notif) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not connected"})
			return
		}
	} else {
		a.nodes.BroadcastNotification(notif)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
