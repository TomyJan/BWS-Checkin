package config

import "testing"

func TestLoadOAuthProvidersFromJSON(t *testing.T) {
	t.Setenv("BWS_DEV_AUTH", "1")
	t.Setenv("BWS_OAUTH_PROVIDERS", `[{"id":"qq","name":"QQ 登录","type":"qq","clientId":"qq-client","clientSecret":"qq-secret","redirectUrl":"https://bws.example.com/api/v1/auth/oauth/qq/callback"}]`)

	cfg := Load()
	if len(cfg.OAuthProviders) != 1 {
		t.Fatalf("providers length = %d, want 1", len(cfg.OAuthProviders))
	}
	provider := cfg.OAuthProviders[0]
	if provider.ID != "qq" || provider.Name != "QQ 登录" || provider.Type != "qq" || provider.ClientID != "qq-client" {
		t.Fatalf("provider = %+v", provider)
	}
}

func TestValidateProductionQQAndCasdoorProviders(t *testing.T) {
	cfg := Config{
		DevAuth:              false,
		PublicBase:           "https://bws.example.com",
		SessionSecret:        "session-secret-for-test",
		BilibiliLoginEnabled: true,
		BilibiliCookieSecret: "bilibili-cookie-secret-for-test",
		OAuthProviders: []OAuthProviderConfig{
			{
				ID:           "qq",
				Name:         "QQ登录",
				Type:         "qq",
				AuthURL:      "https://graph.qq.com/oauth2.0/authorize",
				TokenURL:     "https://graph.qq.com/oauth2.0/token",
				UserInfoURL:  "https://graph.qq.com/user/get_user_info",
				ClientID:     "qq-client-id",
				ClientSecret: "qq-client-secret",
				RedirectURL:  "https://bws.example.com/api/v1/auth/oauth/qq/callback",
			},
			{
				ID:           "casdoor",
				Name:         "AMOE认证",
				Type:         "oidc",
				IssuerURL:    "https://auth.amoe.cc",
				ClientID:     "casdoor-client-id",
				ClientSecret: "casdoor-client-secret",
				RedirectURL:  "https://bws.example.com/api/v1/auth/oauth/casdoor/callback",
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() with QQ and Casdoor providers: %v", err)
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

	cfg.OAuthProviders = []OAuthProviderConfig{{
		ID:           "oidc",
		Name:         "统一认证",
		Type:         "oidc",
		IssuerURL:    "https://issuer.example.com",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://bws.example.com/api/v1/auth/oauth/oidc/callback",
	}}
	cfg.SessionSecret = "production-secret"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() with production config: %v", err)
	}
}
