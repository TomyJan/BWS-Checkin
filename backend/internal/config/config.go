package config

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
)

const developmentBilibiliCookieSecret = "local-development-bilibili-cookie-secret"

type Config struct {
	Addr                 string
	DBPath               string
	UploadDir            string
	DevAuth              bool
	PublicBase           string
	SessionSecret        string
	CookieSecure         bool
	CookieSameSite       string
	SessionMaxAge        int
	BilibiliLoginEnabled bool
	BilibiliCookieSecret string
	BilibiliPassportBase string
	BilibiliAPIBase      string
	OAuthProviders       []OAuthProviderConfig
	OAuthProvidersError  string
}

type OAuthProviderConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	IssuerURL    string `json:"issuerUrl"`
	AuthURL      string `json:"authUrl"`
	TokenURL     string `json:"tokenUrl"`
	UserInfoURL  string `json:"userInfoUrl"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	RedirectURL  string `json:"redirectUrl"`
}

func Load() Config {
	devAuth := env("BWS_DEV_AUTH", "1") == "1"
	bilibiliCookieSecret := env("BWS_BILIBILI_COOKIE_SECRET", "")
	if devAuth && bilibiliCookieSecret == "" {
		bilibiliCookieSecret = developmentBilibiliCookieSecret
	}

	cfg := Config{
		Addr:                 env("BWS_ADDR", ":8080"),
		DBPath:               env("BWS_DB", "data/bws.db"),
		UploadDir:            env("BWS_UPLOAD_DIR", "data/uploads"),
		DevAuth:              devAuth,
		PublicBase:           env("BWS_PUBLIC_BASE", "http://localhost:5173"),
		SessionSecret:        env("BWS_SESSION_SECRET", ""),
		CookieSecure:         env("BWS_COOKIE_SECURE", "0") == "1",
		CookieSameSite:       env("BWS_COOKIE_SAMESITE", "lax"),
		SessionMaxAge:        intEnv("BWS_SESSION_MAX_AGE", 60*60*24*30),
		BilibiliLoginEnabled: env("BWS_BILIBILI_LOGIN_ENABLED", "1") == "1",
		BilibiliCookieSecret: bilibiliCookieSecret,
		BilibiliPassportBase: env("BWS_BILIBILI_PASSPORT_BASE", ""),
		BilibiliAPIBase:      env("BWS_BILIBILI_API_BASE", ""),
	}
	providers, providersErr := loadOAuthProviders(cfg)
	cfg.OAuthProviders = providers
	if providersErr != nil {
		cfg.OAuthProvidersError = providersErr.Error()
	}
	return cfg
}

func (c Config) Validate() error {
	if c.DevAuth {
		return nil
	}
	if c.SessionSecret == "" {
		return errors.New("BWS_SESSION_SECRET is required when BWS_DEV_AUTH=0")
	}
	if c.OAuthProvidersError != "" {
		return errors.New("BWS_OAUTH_PROVIDERS must be valid JSON: " + c.OAuthProvidersError)
	}
	providers := c.OAuthProviders
	if len(providers) == 0 {
		return errors.New("at least one OAuth provider is required when BWS_DEV_AUTH=0")
	}
	for _, provider := range providers {
		if provider.ID == "" || provider.ClientID == "" || provider.ClientSecret == "" || provider.RedirectURL == "" {
			return errors.New("OAuth provider ID, client ID, client secret and redirect URL are required when BWS_DEV_AUTH=0")
		}
		switch provider.Type {
		case "qq":
			if provider.AuthURL == "" || provider.TokenURL == "" || provider.UserInfoURL == "" {
				return errors.New("QQ OAuth auth URL, token URL and userinfo URL are required when BWS_DEV_AUTH=0")
			}
		default:
			if provider.IssuerURL == "" {
				return errors.New("OIDC issuer is required when BWS_DEV_AUTH=0")
			}
		}
	}
	if c.BilibiliLoginEnabled && c.BilibiliCookieSecret == "" {
		return errors.New("BWS_BILIBILI_COOKIE_SECRET is required when Bilibili login is enabled")
	}
	return nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func loadOAuthProviders(cfg Config) ([]OAuthProviderConfig, error) {
	var providers []OAuthProviderConfig
	if raw := env("BWS_OAUTH_PROVIDERS", ""); raw != "" {
		if err := json.Unmarshal([]byte(raw), &providers); err != nil {
			return nil, err
		}
	}
	for index := range providers {
		if providers[index].Type == "" {
			providers[index].Type = "oidc"
		}
		if providers[index].Name == "" {
			providers[index].Name = providers[index].ID
		}
		if providers[index].RedirectURL == "" && cfg.PublicBase != "" && providers[index].ID != "" {
			providers[index].RedirectURL = cfg.PublicBase + "/api/v1/auth/oauth/" + providers[index].ID + "/callback"
		}
	}
	return providers, nil
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
