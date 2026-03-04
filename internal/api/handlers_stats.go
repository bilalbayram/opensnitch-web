package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (a *API) handleGetGeneralStats(w http.ResponseWriter, r *http.Request) {
	// Aggregate stats from all connected nodes
	nodes := a.nodes.GetAllNodes()

	var totalConns, totalDropped, totalAccepted, totalIgnored, totalRules uint64
	for _, node := range nodes {
		stats := node.GetStats()
		if stats == nil {
			continue
		}
		totalConns += stats.Connections
		totalDropped += stats.Dropped
		totalAccepted += stats.Accepted
		totalIgnored += stats.Ignored
		totalRules += stats.Rules
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"nodes_online":  a.nodes.Count(),
		"connections":   totalConns,
		"dropped":       totalDropped,
		"accepted":      totalAccepted,
		"ignored":       totalIgnored,
		"rules":         totalRules,
		"ws_clients":    a.hub.ClientCount(),
	})
}

func (a *API) handleGetStats(w http.ResponseWriter, r *http.Request) {
	table := chi.URLParam(r, "table")

	allowed := map[string]bool{"hosts": true, "procs": true, "addrs": true, "ports": true, "users": true, "processes": true, "addresses": true}
	// Map friendly names to DB table names
	tableMap := map[string]string{
		"hosts":     "hosts",
		"processes": "procs",
		"addresses": "addrs",
		"ports":     "ports",
		"users":     "users",
	}
	if mapped, ok := tableMap[table]; ok {
		table = mapped
	}

	if !allowed[table] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid stats table"})
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	entries, err := a.db.GetStats(table, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, entries)
}
