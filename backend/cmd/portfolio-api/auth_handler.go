package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/schtvr/morgans-d-stonks/internal/auth"
	"github.com/schtvr/morgans-d-stonks/internal/portfolio"
)

func (a *app) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req portfolio.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Username != a.cfg.AuthUsername || req.Password != a.cfg.AuthPassword {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	token, err := auth.NewSessionToken()
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	exp := time.Now().Add(a.cfg.SessionTTL)
	if err := a.repo.CreateSession(r.Context(), token, req.Username, exp); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil || isForwardedHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  exp,
	})
	w.Header().Set("Content-Type", "application/json")
	resp := portfolio.LoginResponse{}
	if !looksLikeBrowserLogin(r) {
		resp.Token = token
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (a *app) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := extractBearerOrCookie(r)
	if token != "" {
		_ = a.repo.DeleteSession(r.Context(), token)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil || isForwardedHTTPS(r),
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func looksLikeBrowserLogin(r *http.Request) bool {
	secPurpose := r.Header.Get("Sec-Purpose")
	if secPurpose == "prefetch" {
		return false
	}
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html") || strings.Contains(accept, "*/*")
}

func isForwardedHTTPS(r *http.Request) bool {
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}
	if xf := r.Header.Get("Forwarded"); xf != "" {
		for _, part := range strings.Split(xf, ",") {
			for _, token := range strings.Split(part, ";") {
				token = strings.TrimSpace(strings.ToLower(token))
				if strings.HasPrefix(token, "proto=") {
					v := strings.Trim(strings.TrimPrefix(token, "proto="), `"`)
					if v == "https" {
						return true
					}
				}
			}
		}
	}
	return false
}

func extractBearerOrCookie(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if len(authHeader) > 7 && strings.EqualFold(authHeader[:7], "Bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	c, err := r.Cookie("session")
	if err == nil {
		return c.Value
	}
	return ""
}
