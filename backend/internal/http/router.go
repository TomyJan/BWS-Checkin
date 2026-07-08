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
		r.Post("/me/qr/upload", h.uploadQR)
		r.Post("/me/qr/delete", h.deleteQR)
		r.Get("/group/list", h.listGroups)
		r.Post("/group/create", h.createGroup)
		r.Get("/group/detail", h.groupDetail)
		r.Post("/group/join", h.joinGroup)
		r.Post("/group/member/remove", h.removeMember)
		r.Get("/group/tasks", h.groupTasks)
		r.Post("/task/complete", h.completeTask)
		r.Post("/task/uncomplete", h.uncompleteTask)
	})
	if deps.UploadDir != "" {
		r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(deps.UploadDir))))
	}
	return r
}
