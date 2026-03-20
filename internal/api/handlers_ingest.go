package api

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/bilalbayram/opensnitch-web/internal/db"
	"github.com/bilalbayram/opensnitch-web/internal/ws"
)

type ingestEvent struct {
	Protocol string `json:"protocol"`
	SrcIP    string `json:"src_ip"`
	SrcPort  int    `json:"src_port"`
	DstIP    string `json:"dst_ip"`
	DstHost  string `json:"dst_host"`
	DstPort  int    `json:"dst_port"`
}

type ingestRequest struct {
	Events []ingestEvent `json:"events"`
}

func (a *API) handleIngest(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing X-API-Key header"})
		return
	}

	router, err := a.db.GetRouterByAPIKey(apiKey)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid API key"})
		return
	}

	var req ingestRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	accepted := 0

	for _, evt := range req.Events {
		record := &db.Connection{
			Time:     now,
			Node:     router.Addr,
			Action:   "allow",
			Protocol: evt.Protocol,
			SrcIP:    evt.SrcIP,
			SrcPort:  evt.SrcPort,
			DstIP:    evt.DstIP,
			DstHost:  evt.DstHost,
			DstPort:  evt.DstPort,
			Rule:     "router-allow",
		}

		if err := a.db.InsertConnection(record); err != nil {
			log.Printf("[ingest] Failed to store connection from %s: %v", router.Addr, err)
			continue
		}

		if record.DstHost != "" && record.DstIP != "" && record.DstPort != 53 {
			if err := a.db.UpsertDNSDomain(router.Addr, record.DstHost, record.DstIP, now); err != nil {
				log.Printf("[ingest] Failed to upsert DNS for %s: %v", router.Addr, err)
			}
		}

		a.hub.BroadcastEvent(ws.EventConnectionEvent, map[string]interface{}{
			"time":         record.Time,
			"node":         record.Node,
			"action":       record.Action,
			"rule":         record.Rule,
			"protocol":     record.Protocol,
			"src_ip":       record.SrcIP,
			"src_port":     record.SrcPort,
			"dst_ip":       record.DstIP,
			"dst_host":     record.DstHost,
			"dst_port":     record.DstPort,
			"uid":          0,
			"pid":          0,
			"process":      "",
			"process_args": []string{},
		})

		// Update stats tables
		if evt.DstHost != "" {
			a.db.UpsertStat("hosts", evt.DstHost, router.Addr, 1)
		}
		if evt.DstIP != "" {
			a.db.UpsertStat("addrs", evt.DstIP, router.Addr, 1)
		}
		if evt.DstPort > 0 {
			a.db.UpsertStat("ports", fmt.Sprintf("%d", evt.DstPort), router.Addr, 1)
		}

		accepted++
	}

	// Keep node heartbeat alive
	a.db.UpsertNode(&db.Node{
		Addr:          router.Addr,
		Hostname:      router.Name,
		DaemonVersion: "conntrack-agent",
		Status:        db.NodeStatusOnline,
		LastConn:      now,
		SourceType:    "router",
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"accepted": accepted,
	})
}
