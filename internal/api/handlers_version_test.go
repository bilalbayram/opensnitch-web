package api

import (
	"net/http"
	"testing"

	"github.com/evilsocket/opensnitch-web/internal/version"
)

func TestHandleGetVersionReturnsBuildInfoOnly(t *testing.T) {
	env := newAPITestEnv(t)

	rec := performJSONRequest(t, env.api.handleGetVersion, http.MethodGet, "/api/v1/version", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	response := decodeJSON[map[string]string](t, rec)
	if response["current_version"] != version.Version {
		t.Fatalf("expected version %q, got %q", version.Version, response["current_version"])
	}
	if _, ok := response["build_time"]; !ok {
		t.Fatalf("expected build_time field in response: %+v", response)
	}
	if _, ok := response["latest_version"]; ok {
		t.Fatalf("did not expect updater fields in response: %+v", response)
	}
}
