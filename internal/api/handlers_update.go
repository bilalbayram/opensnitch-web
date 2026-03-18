package api

import (
	"net/http"

	"github.com/evilsocket/opensnitch-web/internal/version"
)

func (a *API) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"current_version": version.Version,
		"build_time":      version.BuildTime,
	})
}
