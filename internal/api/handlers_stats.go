package api

import (
	"net/http"
	"strconv"

	"github.com/bilalbayram/opensnitch-web/internal/db"
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

	summary, err := a.db.GetConnectionSummary()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"nodes_online": a.nodes.Count(),
		"connections":  totalConns,
		"dropped":      totalDropped,
		"accepted":     totalAccepted,
		"ignored":      totalIgnored,
		"total":        summary.Total,
		"allowed":      summary.Allowed,
		"denied":       summary.Denied,
		"rules":        totalRules,
		"ws_clients":   a.hub.ClientCount(),
	})
}

func (a *API) handleGetTimeSeries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	hours, _ := strconv.Atoi(q.Get("hours"))
	if hours <= 0 {
		hours = 24
	}
	allowedHours := map[int]bool{1: true, 6: true, 24: true, 72: true, 168: true}
	if !allowedHours[hours] {
		hours = 24
	}

	bucketMinutes, _ := strconv.Atoi(q.Get("bucket"))
	allowedBuckets := map[int]bool{1: true, 5: true, 10: true, 15: true, 30: true, 60: true}
	if !allowedBuckets[bucketMinutes] {
		switch {
		case hours <= 1:
			bucketMinutes = 1
		case hours <= 6:
			bucketMinutes = 5
		case hours <= 24:
			bucketMinutes = 15
		default:
			bucketMinutes = 60
		}
	}

	node := q.Get("node")

	points, err := a.db.GetConnectionTimeSeries(hours, bucketMinutes, node)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if points == nil {
		points = []db.TimeSeriesPoint{}
	}

	writeJSON(w, http.StatusOK, points)
}

func (a *API) handleGetTopBlocked(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	dimension := q.Get("dimension")
	if dimension != "hosts" && dimension != "processes" {
		dimension = "hosts"
	}

	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	hours, _ := strconv.Atoi(q.Get("hours"))
	if hours <= 0 {
		hours = 24
	}
	if hours > 168 {
		hours = 168
	}

	entries, err := a.db.GetTopBlocked(dimension, limit, hours)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if entries == nil {
		entries = []db.StatEntry{}
	}

	writeJSON(w, http.StatusOK, entries)
}

func (a *API) handleGeoSummary(w http.ResponseWriter, r *http.Request) {
	if a.geoResolver == nil || !a.geoResolver.Enabled() {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	q := r.URL.Query()
	hours, _ := strconv.Atoi(q.Get("hours"))
	if hours <= 0 {
		hours = 24
	}
	if hours > 168 {
		hours = 168
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	topIPs, err := a.db.GetTopDestinationIPs(limit, hours)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	ips := make([]string, len(topIPs))
	for i, e := range topIPs {
		ips[i] = e.IP
	}

	geoResults := a.geoResolver.LookupBatch(ips)

	type geoEntry struct {
		IP          string  `json:"ip"`
		Country     string  `json:"country"`
		CountryCode string  `json:"country_code"`
		City        string  `json:"city"`
		Lat         float64 `json:"lat"`
		Lon         float64 `json:"lon"`
		Hits        int64   `json:"hits"`
	}

	var results []geoEntry
	for _, ipEntry := range topIPs {
		geo, ok := geoResults[ipEntry.IP]
		if !ok {
			continue
		}
		results = append(results, geoEntry{
			IP:          geo.IP,
			Country:     geo.Country,
			CountryCode: geo.CountryCode,
			City:        geo.City,
			Lat:         geo.Lat,
			Lon:         geo.Lon,
			Hits:        ipEntry.Hits,
		})
	}

	if results == nil {
		results = []geoEntry{}
	}
	writeJSON(w, http.StatusOK, results)
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
