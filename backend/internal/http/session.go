package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
)

const sessionCookieName = "bws_session"

type SessionConfig struct {
	Secret   string
	Secure   bool
	SameSite http.SameSite
	MaxAge   int
}

func (h Handler) setSession(w http.ResponseWriter, userID string) {
	cfg := h.sessionConfig()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    signedSessionValue(userID, cfg.Secret),
		Path:     "/",
		MaxAge:   cfg.MaxAge,
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: cfg.SameSite,
	})
}

func (h Handler) clearSession(w http.ResponseWriter) {
	cfg := h.sessionConfig()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: cfg.SameSite,
	})
}

func (h Handler) sessionUserID(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}
	return parseSignedSessionValue(cookie.Value, h.sessionConfig().Secret)
}

func (h Handler) sessionConfig() SessionConfig {
	cfg := h.deps.Session
	if cfg.Secret == "" {
		cfg.Secret = "dev-session-secret"
	}
	if cfg.SameSite == 0 {
		cfg.SameSite = http.SameSiteLaxMode
	}
	return cfg
}

func signedSessionValue(userID string, secret string) string {
	return userID + "." + signSessionID(userID, secret)
}

func parseSignedSessionValue(value string, secret string) (string, bool) {
	id, signature, ok := strings.Cut(value, ".")
	if !ok || id == "" || signature == "" {
		return "", false
	}
	if !hmac.Equal([]byte(signature), []byte(signSessionID(id, secret))) {
		return "", false
	}
	return id, true
}

func signSessionID(id string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(id))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
