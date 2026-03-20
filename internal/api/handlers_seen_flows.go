package api

import (
	"net/http"
	"strconv"

	"github.com/bilalbayram/opensnitch-web/internal/db"
)

func (a *API) handleGetSeenFlows(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	filter := &db.SeenFlowFilter{
		Node:   q.Get("node"),
		Action: q.Get("action"),
		Search: q.Get("search"),
		Limit:  limit,
		Offset: offset,
	}

	flows, total, err := a.db.GetSeenFlows(filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if flows == nil {
		flows = []db.SeenFlow{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  flows,
		"total": total,
	})
}
