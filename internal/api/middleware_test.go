package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/bilalbayram/opensnitch-web/internal/auth"
	"github.com/bilalbayram/opensnitch-web/internal/config"
)

func TestJWTAuthMiddlewareAllowsWebSocketQueryToken(t *testing.T) {
	cfg := config.DefaultConfig()
	token, err := auth.GenerateToken("admin", &cfg.Auth)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	handler := JWTAuthMiddleware(&cfg.Auth)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws?token="+url.QueryEscape(token), nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected websocket query token to pass, got %d", rec.Code)
	}
}

func TestJWTAuthMiddlewareRejectsQueryTokenOnPlainHTTP(t *testing.T) {
	cfg := config.DefaultConfig()
	token, err := auth.GenerateToken("admin", &cfg.Auth)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	handler := JWTAuthMiddleware(&cfg.Auth)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes?token="+url.QueryEscape(token), nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected plain HTTP query token to be rejected, got %d", rec.Code)
	}
}
