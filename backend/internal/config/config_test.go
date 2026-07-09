package config

import "testing"

func TestLoadOAuthProvidersFromJSON(t *testing.T) {
	t.Setenv("BWS_DEV_AUTH", "1")
	t.Setenv("BWS_OAUTH_PROVIDERS", `[{"id":"qq","name":"QQ 登录","type":"qq","clientId":"qq-client","clientSecret":"qq-secret","redirectUrl":"https://bws.example.com/auth/oauth/qq/callback"}]`)

	cfg := Load()
	if len(cfg.OAuthProviders) != 1 {
		t.Fatalf("providers length = %d, want 1", len(cfg.OAuthProviders))
	}
	provider := cfg.OAuthProviders[0]
	if provider.ID != "qq" || provider.Name != "QQ 登录" || provider.Type != "qq" || provider.ClientID != "qq-client" {
		t.Fatalf("provider = %+v", provider)
	}
}

func TestLoadLegacyOIDCAsOAuthProvider(t *testing.T) {
	t.Setenv("BWS_DEV_AUTH", "1")
	t.Setenv("BWS_OIDC_ISSUER", "https://issuer.example.com")
	t.Setenv("BWS_OIDC_CLIENT_ID", "oidc-client")
	t.Setenv("BWS_OIDC_CLIENT_SECRET", "oidc-secret")
	t.Setenv("BWS_OIDC_NAME", "统一认证")

	cfg := Load()
	if len(cfg.OAuthProviders) != 1 {
		t.Fatalf("providers length = %d, want 1", len(cfg.OAuthProviders))
	}
	provider := cfg.OAuthProviders[0]
	if provider.ID != "oidc" || provider.Name != "统一认证" || provider.Type != "oidc" || provider.IssuerURL != "https://issuer.example.com" {
		t.Fatalf("provider = %+v", provider)
	}
}

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
