package api

import (
	"net/http"
	"strconv"

	"github.com/bilalbayram/opensnitch-web/internal/db"
)

func (a *API) handleGetConnections(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	dstPort, _ := strconv.Atoi(q.Get("dst_port"))

	filter := &db.ConnectionFilter{
		Node:     q.Get("node"),
		Action:   q.Get("action"),
		Protocol: q.Get("protocol"),
		DstHost:  q.Get("dst_host"),
		DstIP:    q.Get("dst_ip"),
		DstPort:  dstPort,
		Process:  q.Get("process"),
		Rule:     q.Get("rule"),
		Search:   q.Get("search"),
		Limit:    limit,
		Offset:   offset,
	}

	conns, total, err := a.db.GetConnections(filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if conns == nil {
		conns = []db.Connection{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  conns,
		"total": total,
	})
}

func (a *API) handlePurgeConnections(w http.ResponseWriter, r *http.Request) {
	if err := a.db.PurgeConnections(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
