package config

import "os"

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
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
