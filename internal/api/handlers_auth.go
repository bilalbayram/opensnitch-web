package api

import (
	"net/http"
	"time"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
	User  string `json:"user"`
}

func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	var hash string
	err := a.db.DB().QueryRow("SELECT password_hash FROM web_users WHERE username = ?", req.Username).Scan(&hash)
	if err != nil || !CheckPassword(req.Password, hash) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	token, err := GenerateToken(req.Username, &a.cfg.Auth)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(a.cfg.Auth.SessionTTL / time.Second),
	})

	writeJSON(w, http.StatusOK, loginResponse{Token: token, User: req.Username})
}

func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   "token",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey)
	writeJSON(w, http.StatusOK, map[string]interface{}{"user": user})
}
