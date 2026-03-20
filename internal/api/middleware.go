package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/bilalbayram/opensnitch-web/internal/auth"
	"github.com/bilalbayram/opensnitch-web/internal/config"
)

type contextKey string

const userContextKey contextKey = "user"

func JWTAuthMiddleware(cfg *config.AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := ""

			// Check Authorization header
			if after, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
				tokenStr = after
			}

			// Check cookie fallback
			if tokenStr == "" {
				if cookie, err := r.Cookie("token"); err == nil {
					tokenStr = cookie.Value
				}
			}

			// WebSocket clients cannot reliably set Authorization headers on the
			// native browser API, so allow the token query param only on upgrades.
			if tokenStr == "" && websocket.IsWebSocketUpgrade(r) && r.URL.Path == "/api/v1/ws" {
				tokenStr = r.URL.Query().Get("token")
			}

			if tokenStr == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}

			claims, err := auth.ValidateToken(tokenStr, cfg)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, claims.Username)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[http] %s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
