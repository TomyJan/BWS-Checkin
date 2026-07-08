package httpapi

import (
	"encoding/json"
	"net/http"

	"bws-checkin/backend/internal/domain"
	"bws-checkin/backend/internal/store"
)

type Deps struct {
	Store   *store.Store
	DevAuth bool
}

type Handler struct {
	deps Deps
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func (h Handler) devLogin(w http.ResponseWriter, r *http.Request) {
	if !h.deps.DevAuth {
		http.NotFound(w, r)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "TomyJan"
	}
	user, err := h.deps.Store.UpsertUser(r.Context(), "dev:"+name, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	setSession(w, user.ID)
	writeJSON(w, http.StatusOK, map[string]domain.User{"user": user})
}

func (h Handler) logout(w http.ResponseWriter, r *http.Request) {
	clearSession(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h Handler) me(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]domain.User{"user": user})
}

func (h Handler) currentUser(w http.ResponseWriter, r *http.Request) (domain.User, bool) {
	userID, ok := sessionUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "login required")
		return domain.User{}, false
	}
	user, err := h.deps.Store.UserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "login required")
		return domain.User{}, false
	}
	return user, true
}
