package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	Addr             string
	DBPath           string
	UploadDir        string
	DevAuth          bool
	PublicBase       string
	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string
	SessionSecret    string
	CookieSecure     bool
	CookieSameSite   string
	SessionMaxAge    int
}

func Load() Config {
	return Config{
		Addr:             env("BWS_ADDR", ":8080"),
		DBPath:           env("BWS_DB", "data/bws.db"),
		UploadDir:        env("BWS_UPLOAD_DIR", "data/uploads"),
		DevAuth:          env("BWS_DEV_AUTH", "1") == "1",
		PublicBase:       env("BWS_PUBLIC_BASE", "http://localhost:5173"),
		OIDCIssuerURL:    env("BWS_OIDC_ISSUER", ""),
		OIDCClientID:     env("BWS_OIDC_CLIENT_ID", ""),
		OIDCClientSecret: env("BWS_OIDC_CLIENT_SECRET", ""),
		OIDCRedirectURL:  env("BWS_OIDC_REDIRECT_URL", ""),
		SessionSecret:    env("BWS_SESSION_SECRET", ""),
		CookieSecure:     env("BWS_COOKIE_SECURE", "0") == "1",
		CookieSameSite:   env("BWS_COOKIE_SAMESITE", "lax"),
		SessionMaxAge:    intEnv("BWS_SESSION_MAX_AGE", 60*60*24*30),
	}
}

func (c Config) Validate() error {
	if c.DevAuth {
		return nil
	}
	if c.SessionSecret == "" {
		return errors.New("BWS_SESSION_SECRET is required when BWS_DEV_AUTH=0")
	}
	if c.OIDCIssuerURL == "" || c.OIDCClientID == "" || c.OIDCClientSecret == "" {
		return errors.New("OIDC issuer, client ID and client secret are required when BWS_DEV_AUTH=0")
	}
	return nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
