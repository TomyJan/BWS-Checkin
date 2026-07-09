package httpapi

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"bws-checkin/backend/internal/domain"
	"bws-checkin/backend/internal/filestore"
	"bws-checkin/backend/internal/store"
	_ "golang.org/x/image/webp"
)

type Deps struct {
	Store     *store.Store
	DevAuth   bool
	UploadDir string
	OIDC      OIDCConfig
	Session   SessionConfig
}

type Handler struct {
	deps Deps
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type apiResponse struct {
	OK    bool      `json:"ok"`
	Data  any       `json:"data,omitempty"`
	Error *apiError `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeOK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: data})
}

func writeBusinessError(w http.ResponseWriter, code string, message string) {
	writeJSON(w, http.StatusOK, apiResponse{OK: false, Error: &apiError{Code: code, Message: message}})
}

func writeUnauthorized(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, apiResponse{OK: false, Error: &apiError{Code: "login_required", Message: "login required"}})
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
		writeBusinessError(w, "user_upsert_failed", err.Error())
		return
	}
	h.setSession(w, user.ID)
	writeOK(w, map[string]domain.User{"user": user})
}

func (h Handler) logout(w http.ResponseWriter, r *http.Request) {
	h.clearSession(w)
	writeOK(w, map[string]bool{"ok": true})
}

func (h Handler) me(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	writeOK(w, map[string]domain.User{"user": user})
}

func (h Handler) uploadQR(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		writeBusinessError(w, "invalid_upload", "invalid upload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeBusinessError(w, "file_required", "file is required")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedImageExt(ext) {
		writeBusinessError(w, "unsupported_image_type", "unsupported image type")
		return
	}
	body, err := io.ReadAll(file)
	if err != nil {
		writeBusinessError(w, "invalid_upload", "invalid upload")
		return
	}
	if !validImageContent(ext, body) {
		writeBusinessError(w, "invalid_image", "invalid image")
		return
	}
	files := filestore.Local{Dir: h.deps.UploadDir}
	url, err := files.SaveQR(user.ID, ext, bytes.NewReader(body))
	if err != nil {
		writeBusinessError(w, "qr_save_failed", err.Error())
		return
	}
	if err := h.deps.Store.UpdateUserQR(r.Context(), user.ID, url); err != nil {
		writeBusinessError(w, "qr_update_failed", err.Error())
		return
	}
	if user.QRImageURL != "" && user.QRImageURL != url {
		_ = files.DeleteURL(user.QRImageURL)
	}
	h.audit(r, user.ID, "qr.upload", "", user.ID, "")
	updated, err := h.deps.Store.UserByID(r.Context(), user.ID)
	if err != nil {
		writeBusinessError(w, "current_user_load_failed", err.Error())
		return
	}
	writeOK(w, map[string]domain.User{"user": updated})
}

func (h Handler) deleteQR(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	_ = filestore.Local{Dir: h.deps.UploadDir}.DeleteURL(user.QRImageURL)
	if err := h.deps.Store.UpdateUserQR(r.Context(), user.ID, ""); err != nil {
		writeBusinessError(w, "qr_delete_failed", err.Error())
		return
	}
	h.audit(r, user.ID, "qr.delete", "", user.ID, "")
	writeOK(w, map[string]bool{"ok": true})
}

type createGroupRequest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Day         string `json:"day"`
	Description string `json:"description"`
}

type updateGroupRequest struct {
	GroupID     string `json:"groupId"`
	Name        string `json:"name"`
	Day         string `json:"day"`
	Description string `json:"description"`
}

func (h Handler) listGroups(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	includeArchived := r.URL.Query().Get("includeArchived") == "1" || r.URL.Query().Get("includeArchived") == "true"
	groups, err := h.deps.Store.UserGroups(r.Context(), user.ID, includeArchived)
	if err != nil {
		writeBusinessError(w, "groups_load_failed", err.Error())
		return
	}
	writeOK(w, map[string][]domain.Group{"groups": groups})
}

func (h Handler) createGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	var input createGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeBusinessError(w, "invalid_json", "invalid JSON")
		return
	}
	if input.ID == "" || input.Name == "" || input.Day == "" {
		writeBusinessError(w, "invalid_group_input", "id, name and day are required")
		return
	}
	err := h.deps.Store.CreateGroup(r.Context(), store.CreateGroupInput{
		ID: input.ID, Name: input.Name, Day: input.Day, Description: input.Description, OwnerUserID: user.ID,
	})
	if err != nil {
		writeBusinessError(w, "group_id_conflict", err.Error())
		return
	}
	group, err := h.deps.Store.GroupByID(r.Context(), input.ID, user.ID)
	if err != nil {
		writeBusinessError(w, "group_load_failed", err.Error())
		return
	}
	writeOK(w, map[string]domain.Group{"group": group})
}

func (h Handler) updateGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	var input updateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeBusinessError(w, "invalid_json", "invalid JSON")
		return
	}
	if input.GroupID == "" || input.Name == "" || input.Day == "" {
		writeBusinessError(w, "invalid_group_input", "groupId, name and day are required")
		return
	}
	if !h.requireOwner(w, r, input.GroupID, user.ID) {
		return
	}
	if err := h.deps.Store.UpdateGroup(r.Context(), store.UpdateGroupInput{
		ID: input.GroupID, Name: input.Name, Day: input.Day, Description: input.Description,
	}); err != nil {
		writeBusinessError(w, "group_update_failed", err.Error())
		return
	}
	h.audit(r, user.ID, "group.update", input.GroupID, "", "")
	group, err := h.deps.Store.GroupByID(r.Context(), input.GroupID, user.ID)
	if err != nil {
		writeNotFoundOrForbidden(w, err)
		return
	}
	writeOK(w, map[string]domain.Group{"group": group})
}

func (h Handler) lockGroupJoin(w http.ResponseWriter, r *http.Request) {
	h.setGroupJoinLocked(w, r, true)
}

func (h Handler) unlockGroupJoin(w http.ResponseWriter, r *http.Request) {
	h.setGroupJoinLocked(w, r, false)
}

func (h Handler) setGroupJoinLocked(w http.ResponseWriter, r *http.Request, locked bool) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	var input groupIDRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeBusinessError(w, "invalid_json", "invalid JSON")
		return
	}
	if !h.requireOwner(w, r, input.GroupID, user.ID) {
		return
	}
	if err := h.deps.Store.SetGroupJoinLocked(r.Context(), input.GroupID, locked); err != nil {
		writeBusinessError(w, "group_join_lock_failed", err.Error())
		return
	}
	action := "group.join_unlock"
	if locked {
		action = "group.join_lock"
	}
	h.audit(r, user.ID, action, input.GroupID, "", "")
	group, err := h.deps.Store.GroupByID(r.Context(), input.GroupID, user.ID)
	if err != nil {
		writeNotFoundOrForbidden(w, err)
		return
	}
	writeOK(w, map[string]domain.Group{"group": group})
}

func (h Handler) archiveGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	var input groupIDRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeBusinessError(w, "invalid_json", "invalid JSON")
		return
	}
	if !h.requireOwner(w, r, input.GroupID, user.ID) {
		return
	}
	if err := h.deps.Store.ArchiveGroup(r.Context(), input.GroupID); err != nil {
		writeBusinessError(w, "group_archive_failed", err.Error())
		return
	}
	h.audit(r, user.ID, "group.archive", input.GroupID, "", "")
	group, err := h.deps.Store.GroupByID(r.Context(), input.GroupID, user.ID)
	if err != nil {
		writeNotFoundOrForbidden(w, err)
		return
	}
	writeOK(w, map[string]domain.Group{"group": group})
}

func (h Handler) groupDetail(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	group, err := h.deps.Store.GroupByID(r.Context(), r.URL.Query().Get("groupId"), user.ID)
	if err != nil {
		writeNotFoundOrForbidden(w, err)
		return
	}
	writeOK(w, map[string]domain.Group{"group": group})
}

func (h Handler) joinGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	var input groupIDRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeBusinessError(w, "invalid_json", "invalid JSON")
		return
	}
	groupID := input.GroupID
	if err := h.deps.Store.JoinGroup(r.Context(), groupID, user.ID); err != nil {
		if errors.Is(err, store.ErrGroupArchived) {
			writeBusinessError(w, "group_archived", err.Error())
			return
		}
		if errors.Is(err, store.ErrGroupJoinLocked) {
			writeBusinessError(w, "group_join_locked", err.Error())
			return
		}
		writeBusinessError(w, "group_join_failed", err.Error())
		return
	}
	group, err := h.deps.Store.GroupByID(r.Context(), groupID, user.ID)
	if err != nil {
		writeNotFoundOrForbidden(w, err)
		return
	}
	writeOK(w, map[string]domain.Group{"group": group})
}

func (h Handler) removeMember(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	var input memberActionRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeBusinessError(w, "invalid_json", "invalid JSON")
		return
	}
	groupID := input.GroupID
	owner, err := h.deps.Store.IsOwner(r.Context(), groupID, user.ID)
	if err != nil {
		writeBusinessError(w, "owner_check_failed", err.Error())
		return
	}
	if !owner {
		writeBusinessError(w, "owner_role_required", "owner role required")
		return
	}
	if input.UserID == user.ID {
		writeBusinessError(w, "owner_remove_forbidden", "owner cannot remove self")
		return
	}
	if err := h.deps.Store.RemoveMember(r.Context(), groupID, input.UserID); err != nil {
		writeBusinessError(w, "member_remove_failed", err.Error())
		return
	}
	h.audit(r, user.ID, "group.member_remove", groupID, input.UserID, "")
	writeOK(w, map[string]bool{"ok": true})
}

func (h Handler) groupTasks(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	groupID := r.URL.Query().Get("groupId")
	if _, err := h.deps.Store.GroupByID(r.Context(), groupID, user.ID); err != nil {
		writeNotFoundOrForbidden(w, err)
		return
	}
	tasks, err := h.deps.Store.GroupTasks(r.Context(), groupID)
	if err != nil {
		writeBusinessError(w, "tasks_load_failed", err.Error())
		return
	}
	writeOK(w, map[string][]domain.TaskStatus{"tasks": tasks})
}

func (h Handler) completeTask(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	var input taskCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeBusinessError(w, "invalid_json", "invalid JSON")
		return
	}
	groupID := input.GroupID
	if _, err := h.deps.Store.GroupByID(r.Context(), groupID, user.ID); err != nil {
		writeNotFoundOrForbidden(w, err)
		return
	}
	if err := h.deps.Store.SyncTaskCompletion(r.Context(), store.SyncTaskCompletionInput{
		GroupID:         groupID,
		TaskID:          input.TaskID,
		TargetUserID:    input.UserID,
		CheckedByUserID: user.ID,
		Completed:       true,
		UpdatedAt:       input.syncTime(),
	}); err != nil {
		if errors.Is(err, store.ErrGroupArchived) {
			writeBusinessError(w, "group_archived", err.Error())
			return
		}
		writeBusinessError(w, "task_complete_failed", err.Error())
		return
	}
	h.audit(r, user.ID, "task.complete", groupID, input.UserID, input.TaskID)
	writeOK(w, map[string]bool{"ok": true})
}

func (h Handler) uncompleteTask(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	var input taskCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeBusinessError(w, "invalid_json", "invalid JSON")
		return
	}
	groupID := input.GroupID
	if _, err := h.deps.Store.GroupByID(r.Context(), groupID, user.ID); err != nil {
		writeNotFoundOrForbidden(w, err)
		return
	}
	if err := h.deps.Store.SyncTaskCompletion(r.Context(), store.SyncTaskCompletionInput{
		GroupID:         groupID,
		TaskID:          input.TaskID,
		TargetUserID:    input.UserID,
		CheckedByUserID: user.ID,
		Completed:       false,
		UpdatedAt:       input.syncTime(),
	}); err != nil {
		if errors.Is(err, store.ErrGroupArchived) {
			writeBusinessError(w, "group_archived", err.Error())
			return
		}
		writeBusinessError(w, "task_uncomplete_failed", err.Error())
		return
	}
	h.audit(r, user.ID, "task.uncomplete", groupID, input.UserID, input.TaskID)
	writeOK(w, map[string]bool{"ok": true})
}

func (h Handler) currentUser(w http.ResponseWriter, r *http.Request) (domain.User, bool) {
	userID, ok := h.sessionUserID(r)
	if !ok {
		writeUnauthorized(w)
		return domain.User{}, false
	}
	user, err := h.deps.Store.UserByID(r.Context(), userID)
	if err != nil {
		writeUnauthorized(w)
		return domain.User{}, false
	}
	return user, true
}

func (h Handler) requireOwner(w http.ResponseWriter, r *http.Request, groupID string, userID string) bool {
	owner, err := h.deps.Store.IsOwner(r.Context(), groupID, userID)
	if err != nil {
		writeBusinessError(w, "owner_check_failed", err.Error())
		return false
	}
	if !owner {
		writeBusinessError(w, "owner_role_required", "owner role required")
		return false
	}
	return true
}

func (h Handler) audit(r *http.Request, actorUserID string, action string, groupID string, targetUserID string, taskID string) {
	_ = h.deps.Store.AppendAuditLog(r.Context(), store.AuditLogInput{
		ActorUserID:  actorUserID,
		Action:       action,
		GroupID:      groupID,
		TargetUserID: targetUserID,
		TaskID:       taskID,
	})
}

func allowedImageExt(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
		return true
	default:
		return false
	}
}

func validImageContent(ext string, body []byte) bool {
	_, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		return false
	}
	switch ext {
	case ".png":
		return format == "png"
	case ".jpg", ".jpeg":
		return format == "jpeg"
	case ".webp":
		return format == "webp"
	default:
		return false
	}
}

func writeNotFoundOrForbidden(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeBusinessError(w, "group_access_denied", "group access denied")
		return
	}
	writeBusinessError(w, "request_failed", err.Error())
}

type groupIDRequest struct {
	GroupID string `json:"groupId"`
}

type memberActionRequest struct {
	GroupID string `json:"groupId"`
	UserID  string `json:"userId"`
}

type taskCompletionRequest struct {
	GroupID   string     `json:"groupId"`
	TaskID    string     `json:"taskId"`
	UserID    string     `json:"userId"`
	UpdatedAt *time.Time `json:"updatedAt"`
}

func (r taskCompletionRequest) syncTime() time.Time {
	if r.UpdatedAt != nil {
		return r.UpdatedAt.UTC()
	}
	return time.Now().UTC()
}
