package api

import "net/http"

func (a *API) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.updater.Status())
}

func (a *API) handleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	status, err := a.updater.CheckNow()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *API) handleApplyUpdate(w http.ResponseWriter, r *http.Request) {
	if err := a.updater.Apply(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updating"})
}
