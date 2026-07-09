package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"bws-checkin/backend/internal/frontend"
	"github.com/go-chi/chi/v5"
)

func NewRouter(deps Deps) http.Handler {
	return NewRouterWithLogger(deps, slog.Default())
}

func NewRouterWithLogger(deps Deps, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	r := chi.NewRouter()
	r.Use(requestLogger(logger))
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
		r.Post("/me/qr/source/set", h.setQRSource)
		r.Get("/user/qr", h.userQR)
		r.Get("/bilibili/account", h.bilibiliAccount)
		r.Post("/bilibili/login/qrcode/create", h.createBilibiliLoginQRCode)
		r.Post("/bilibili/login/qrcode/poll", h.pollBilibiliLoginQRCode)
		r.Post("/bilibili/account/unbind", h.unbindBilibiliAccount)
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
		r.Get("/task/sync/status", h.taskSyncStatus)
		r.Post("/task/sync", h.syncTasks)
		r.Post("/task/complete", h.completeTask)
		r.Post("/task/uncomplete", h.uncompleteTask)
	})
	r.NotFound(frontend.Handler().ServeHTTP)
	return r
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(body)
	r.bytes += n
	return n, err
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			recorder := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(recorder, r)
			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}
			logger.InfoContext(r.Context(), "http_request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Int("bytes", recorder.bytes),
				slog.Int64("duration_ms", time.Since(startedAt).Milliseconds()),
				slog.String("remote_addr", r.RemoteAddr),
			)
		})
	}
}
