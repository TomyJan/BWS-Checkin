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
	return httpapi.NewRouter(httpapi.Deps{Store: db, DevAuth: cfg.DevAuth}), cleanup, nil
}
