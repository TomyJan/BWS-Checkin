package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	"bws-checkin/backend/internal/bilibili"
	"bws-checkin/backend/internal/config"
	httpapi "bws-checkin/backend/internal/http"
	"bws-checkin/backend/internal/store"
	"bws-checkin/backend/internal/tasksync"
)

func New(cfg config.Config) (http.Handler, func(), error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	cleanup := func() {
		cancel()
		_ = db.Close()
	}
	var bilibiliClient *bilibili.Client
	if cfg.BilibiliLoginEnabled {
		bilibiliClient = bilibili.NewClient(bilibili.ClientOptions{
			PassportBaseURL: cfg.BilibiliPassportBase,
			APIBaseURL:      cfg.BilibiliAPIBase,
		})
	}
	var taskSync *tasksync.Syncer
	if bilibiliClient != nil {
		taskSync = tasksync.New(db, tasksync.NewBilibiliSource(tasksync.BilibiliSourceConfig{
			Store:        db,
			Client:       bilibiliClient,
			CookieSecret: cfg.BilibiliCookieSecret,
		}), tasksync.Config{FreshTTL: 5 * time.Minute})
		go runTaskSync(ctx, taskSync)
	}
	return httpapi.NewRouter(httpapi.Deps{
		Store:                db,
		DevAuth:              cfg.DevAuth,
		UploadDir:            cfg.UploadDir,
		Bilibili:             bilibiliClient,
		BilibiliCookieSecret: cfg.BilibiliCookieSecret,
		TaskSync:             taskSync,
		OAuthProviders:       oauthProviders(cfg),
		Session: httpapi.SessionConfig{
			Secret:   cfg.SessionSecret,
			Secure:   cfg.CookieSecure,
			SameSite: sameSite(cfg.CookieSameSite),
			MaxAge:   cfg.SessionMaxAge,
		},
	}), cleanup, nil
}

func oauthProviders(cfg config.Config) []httpapi.OAuthProviderConfig {
	providers := make([]httpapi.OAuthProviderConfig, 0, len(cfg.OAuthProviders))
	for _, provider := range cfg.OAuthProviders {
		providers = append(providers, httpapi.OAuthProviderConfig{
			ID:           provider.ID,
			Name:         provider.Name,
			Type:         provider.Type,
			IssuerURL:    provider.IssuerURL,
			AuthURL:      provider.AuthURL,
			TokenURL:     provider.TokenURL,
			UserInfoURL:  provider.UserInfoURL,
			ClientID:     provider.ClientID,
			ClientSecret: provider.ClientSecret,
			RedirectURL:  provider.RedirectURL,
		})
	}
	return providers
}

func runTaskSync(ctx context.Context, syncer *tasksync.Syncer) {
	_ = syncer.Sync(ctx)
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = syncer.Sync(ctx)
		}
	}
}

func sameSite(value string) http.SameSite {
	switch strings.ToLower(value) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
