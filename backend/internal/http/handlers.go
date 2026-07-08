package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"bws-checkin/backend/internal/domain"
	"bws-checkin/backend/internal/store"
	"github.com/go-chi/chi/v5"
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

type createGroupRequest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Day         string `json:"day"`
	Description string `json:"description"`
}

func (h Handler) listGroups(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	groups, err := h.deps.Store.UserGroups(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string][]domain.Group{"groups": groups})
}

func (h Handler) createGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	var input createGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if input.ID == "" || input.Name == "" || input.Day == "" {
		writeError(w, http.StatusBadRequest, "id, name and day are required")
		return
	}
	err := h.deps.Store.CreateGroup(r.Context(), store.CreateGroupInput{
		ID: input.ID, Name: input.Name, Day: input.Day, Description: input.Description, OwnerUserID: user.ID,
	})
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	group, err := h.deps.Store.GroupByID(r.Context(), input.ID, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]domain.Group{"group": group})
}

func (h Handler) groupDetail(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	group, err := h.deps.Store.GroupByID(r.Context(), chi.URLParam(r, "groupId"), user.ID)
	if err != nil {
		writeNotFoundOrForbidden(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]domain.Group{"group": group})
}

func (h Handler) joinGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	groupID := chi.URLParam(r, "groupId")
	if err := h.deps.Store.JoinGroup(r.Context(), groupID, user.ID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	group, err := h.deps.Store.GroupByID(r.Context(), groupID, user.ID)
	if err != nil {
		writeNotFoundOrForbidden(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]domain.Group{"group": group})
}

func (h Handler) removeMember(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	groupID := chi.URLParam(r, "groupId")
	owner, err := h.deps.Store.IsOwner(r.Context(), groupID, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !owner {
		writeError(w, http.StatusForbidden, "owner role required")
		return
	}
	memberID, err := strconv.ParseInt(chi.URLParam(r, "userId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := h.deps.Store.RemoveMember(r.Context(), groupID, memberID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h Handler) groupTasks(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	groupID := chi.URLParam(r, "groupId")
	if _, err := h.deps.Store.GroupByID(r.Context(), groupID, user.ID); err != nil {
		writeNotFoundOrForbidden(w, err)
		return
	}
	tasks, err := h.deps.Store.GroupTasks(r.Context(), groupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string][]domain.TaskStatus{"tasks": tasks})
}

func (h Handler) completeTask(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	groupID := chi.URLParam(r, "groupId")
	targetUserID, err := strconv.ParseInt(chi.URLParam(r, "userId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if _, err := h.deps.Store.GroupByID(r.Context(), groupID, user.ID); err != nil {
		writeNotFoundOrForbidden(w, err)
		return
	}
	if err := h.deps.Store.MarkComplete(r.Context(), groupID, chi.URLParam(r, "taskId"), targetUserID, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h Handler) uncompleteTask(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	groupID := chi.URLParam(r, "groupId")
	targetUserID, err := strconv.ParseInt(chi.URLParam(r, "userId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if _, err := h.deps.Store.GroupByID(r.Context(), groupID, user.ID); err != nil {
		writeNotFoundOrForbidden(w, err)
		return
	}
	if err := h.deps.Store.UnmarkComplete(r.Context(), groupID, chi.URLParam(r, "taskId"), targetUserID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
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

func writeNotFoundOrForbidden(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusForbidden, "group access denied")
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}
