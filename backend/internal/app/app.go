package app

import (
	"net/http"

	"bws-checkin/backend/internal/config"
	httpapi "bws-checkin/backend/internal/http"
)

func New(cfg config.Config) (http.Handler, func(), error) {
	return httpapi.NewRouter(), func() {}, nil
}
