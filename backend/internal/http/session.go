package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
)

const sessionCookieName = "bws_session"

type SessionConfig struct {
	Secret   string
	Secure   bool
	SameSite http.SameSite
	MaxAge   int
}

func (h Handler) setSession(w http.ResponseWriter, userID int64) {
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

func (h Handler) sessionUserID(r *http.Request) (int64, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return 0, false
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

func signedSessionValue(userID int64, secret string) string {
	id := strconv.FormatInt(userID, 10)
	return id + "." + signSessionID(id, secret)
}

func parseSignedSessionValue(value string, secret string) (int64, bool) {
	id, signature, ok := strings.Cut(value, ".")
	if !ok || id == "" || signature == "" {
		return 0, false
	}
	if !hmac.Equal([]byte(signature), []byte(signSessionID(id, secret))) {
		return 0, false
	}
	parsed, err := strconv.ParseInt(id, 10, 64)
	return parsed, err == nil
}

func signSessionID(id string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(id))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
