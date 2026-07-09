package httpapi

import (
	"net/http"

	"bws-checkin/backend/internal/frontend"
	"github.com/go-chi/chi/v5"
)

func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	h := Handler{deps: deps}
	r.Get("/auth/oidc/login", h.oidcLogin)
	r.Get("/auth/oidc/callback", h.oidcCallback)
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/dev/login", h.devLogin)
		r.Post("/logout", h.logout)
		r.Get("/me", h.me)
		r.Post("/me/qr/upload", h.uploadQR)
		r.Post("/me/qr/delete", h.deleteQR)
		r.Get("/user/qr", h.userQR)
		r.Get("/group/list", h.listGroups)
		r.Post("/group/create", h.createGroup)
		r.Post("/group/update", h.updateGroup)
		r.Post("/group/join-lock", h.lockGroupJoin)
		r.Post("/group/join-unlock", h.unlockGroupJoin)
		r.Post("/group/archive", h.archiveGroup)
		r.Get("/group/detail", h.groupDetail)
		r.Post("/group/join", h.joinGroup)
		r.Post("/group/member/remove", h.removeMember)
		r.Get("/group/tasks", h.groupTasks)
		r.Post("/task/complete", h.completeTask)
		r.Post("/task/uncomplete", h.uncompleteTask)
	})
	r.NotFound(frontend.Handler().ServeHTTP)
	return r
}
