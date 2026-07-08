package app

import (
	"net/http"

	"bws-checkin/backend/internal/config"
	httpapi "bws-checkin/backend/internal/http"
	"bws-checkin/backend/internal/store"
)

func New(cfg config.Config) (http.Handler, func(), error) {
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = db.Close() }
	return httpapi.NewRouter(httpapi.Deps{
		Store:     db,
		DevAuth:   cfg.DevAuth,
		UploadDir: cfg.UploadDir,
		OIDC: httpapi.OIDCConfig{
			IssuerURL:         cfg.OIDCIssuerURL,
			ClientID:          cfg.OIDCClientID,
			ClientSecret:      cfg.OIDCClientSecret,
			RedirectURL:       oidcRedirectURL(cfg),
			PostLoginRedirect: cfg.PublicBase,
		},
	}), cleanup, nil
}

func oidcRedirectURL(cfg config.Config) string {
	if cfg.OIDCRedirectURL != "" {
		return cfg.OIDCRedirectURL
	}
	return cfg.PublicBase + "/auth/oidc/callback"
}
