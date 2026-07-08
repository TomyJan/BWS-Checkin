package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	h := Handler{deps: deps}
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/dev/login", h.devLogin)
		r.Post("/logout", h.logout)
		r.Get("/me", h.me)
	})
	return r
}
