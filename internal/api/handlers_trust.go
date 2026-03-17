package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (a *API) handleGetProcessTrust(w http.ResponseWriter, r *http.Request) {
	addr := chi.URLParam(r, "addr")
	list, err := a.db.GetProcessTrustList(addr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if list == nil {
		writeJSON(w, http.StatusOK, []struct{}{})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) handleAddProcessTrust(w http.ResponseWriter, r *http.Request) {
	addr := chi.URLParam(r, "addr")

	var req struct {
		ProcessPath string `json:"process_path"`
		TrustLevel  string `json:"trust_level"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if req.ProcessPath == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "process_path required"})
		return
	}
	if req.TrustLevel != "trusted" && req.TrustLevel != "untrusted" && req.TrustLevel != "default" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "trust_level must be trusted, untrusted, or default"})
		return
	}

	pt, err := a.db.AddProcessTrust(addr, req.ProcessPath, req.TrustLevel)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "entry already exists for this node and process path"})
		return
	}
	writeJSON(w, http.StatusCreated, pt)
}

func (a *API) handleUpdateProcessTrust(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var req struct {
		TrustLevel string `json:"trust_level"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if req.TrustLevel != "trusted" && req.TrustLevel != "untrusted" && req.TrustLevel != "default" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "trust_level must be trusted, untrusted, or default"})
		return
	}

	if err := a.db.UpdateProcessTrust(id, req.TrustLevel); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) handleDeleteProcessTrust(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := a.db.DeleteProcessTrust(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
