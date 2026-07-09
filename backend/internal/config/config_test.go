package config

import "testing"

func TestLoadUsesDevelopmentBilibiliCookieSecretFallback(t *testing.T) {
	t.Setenv("BWS_DEV_AUTH", "1")
	t.Setenv("BWS_BILIBILI_COOKIE_SECRET", "")

	cfg := Load()
	if cfg.BilibiliCookieSecret == "" {
		t.Fatal("BilibiliCookieSecret is empty in development config")
	}
}

func TestLoadDoesNotUseBilibiliCookieSecretFallbackInProduction(t *testing.T) {
	t.Setenv("BWS_DEV_AUTH", "0")
	t.Setenv("BWS_BILIBILI_COOKIE_SECRET", "")

	cfg := Load()
	if cfg.BilibiliCookieSecret != "" {
		t.Fatalf("BilibiliCookieSecret = %q, want empty production config", cfg.BilibiliCookieSecret)
	}
}

func TestValidateProductionRequiresOIDCAndSessionSecret(t *testing.T) {
	cfg := Config{DevAuth: false, PublicBase: "https://bws.example.com"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() err = nil, want missing production config error")
	}

	cfg.OIDCIssuerURL = "https://issuer.example.com"
	cfg.OIDCClientID = "client-id"
	cfg.OIDCClientSecret = "client-secret"
	cfg.SessionSecret = "production-secret"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() with production config: %v", err)
	}
}
