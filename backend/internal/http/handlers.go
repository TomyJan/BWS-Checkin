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
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bws-checkin/backend/internal/bilibili"
	"bws-checkin/backend/internal/domain"
	"bws-checkin/backend/internal/filestore"
	"bws-checkin/backend/internal/qrcode"
	"bws-checkin/backend/internal/store"
	"bws-checkin/backend/internal/tasksync"
	_ "golang.org/x/image/webp"
)

type Deps struct {
	Store                *store.Store
	DevAuth              bool
	UploadDir            string
	OIDC                 OIDCConfig
	OAuthProviders       []OAuthProviderConfig
	Session              SessionConfig
	Bilibili             *bilibili.Client
	BilibiliCookieSecret string
	TaskSync             *tasksync.Syncer
}

type OAuthProviderConfig struct {
	ID           string
	Name         string
	Type         string
	IssuerURL    string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	ClientID     string
	ClientSecret string
	RedirectURL  string
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
	message = businessErrorMessage(code, message)
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

func (h Handler) oauthProviders(w http.ResponseWriter, r *http.Request) {
	providers := make([]domain.OAuthProvider, 0, len(h.deps.OAuthProviders))
	for _, provider := range h.deps.OAuthProviders {
		if provider.ID == "" {
			continue
		}
		providers = append(providers, domain.OAuthProvider{
			ID:   provider.ID,
			Name: provider.displayName(),
			Type: provider.providerType(),
		})
	}
	writeOK(w, map[string][]domain.OAuthProvider{"providers": providers})
}

func (h Handler) oauthAccounts(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	accounts, err := h.deps.Store.UserOAuthAccounts(r.Context(), user.ID)
	if err != nil {
		writeBusinessError(w, "oauth_accounts_load_failed", err.Error())
		return
	}
	for index := range accounts {
		if provider, ok := h.oauthProvider(accounts[index].ProviderID); ok {
			accounts[index].ProviderName = provider.displayName()
		}
	}
	writeOK(w, map[string][]domain.OAuthAccount{"accounts": accounts})
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
	oldQRPath, _ := h.deps.Store.UserQRPath(r.Context(), user.ID)
	url, err := files.SaveQR(user.ID, ext, bytes.NewReader(body))
	if err != nil {
		writeBusinessError(w, "qr_save_failed", err.Error())
		return
	}
	if err := h.deps.Store.UpdateUserQR(r.Context(), user.ID, url); err != nil {
		writeBusinessError(w, "qr_update_failed", err.Error())
		return
	}
	if oldQRPath != "" && oldQRPath != url {
		_ = files.DeleteURL(oldQRPath)
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
	qrPath, _ := h.deps.Store.UserQRPath(r.Context(), user.ID)
	_ = filestore.Local{Dir: h.deps.UploadDir}.DeleteURL(qrPath)
	if err := h.deps.Store.UpdateUserQR(r.Context(), user.ID, ""); err != nil {
		writeBusinessError(w, "qr_delete_failed", err.Error())
		return
	}
	h.audit(r, user.ID, "qr.delete", "", user.ID, "")
	writeOK(w, map[string]bool{"ok": true})
}

type qrSourceSetRequest struct {
	Source string `json:"source"`
}

func (h Handler) setQRSource(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	var input qrSourceSetRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeBusinessError(w, "invalid_json", "invalid JSON")
		return
	}
	if input.Source == domain.QRSourceBilibiliGenerated {
		if _, err := h.deps.Store.BilibiliAccount(r.Context(), user.ID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeBusinessError(w, "bilibili_account_required", "")
				return
			}
			writeBusinessError(w, "bilibili_account_load_failed", err.Error())
			return
		}
	}
	if err := h.deps.Store.SetUserQRSource(r.Context(), user.ID, input.Source); err != nil {
		writeBusinessError(w, "invalid_qr_source", "")
		return
	}
	updated, err := h.deps.Store.UserByID(r.Context(), user.ID)
	if err != nil {
		writeBusinessError(w, "current_user_load_failed", err.Error())
		return
	}
	writeOK(w, map[string]domain.User{"user": updated})
}

func (h Handler) userQR(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentUser(w, r); !ok {
		return
	}
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		writeBusinessError(w, "user_id_required", "userId is required")
		return
	}
	targetUser, err := h.deps.Store.UserByID(r.Context(), userID)
	if err != nil {
		writeBusinessError(w, "qr_not_found", "QR image not found")
		return
	}
	if targetUser.QRSource == domain.QRSourceBilibiliGenerated {
		account, err := h.deps.Store.BilibiliAccount(r.Context(), userID)
		if err != nil {
			writeBusinessError(w, "qr_not_found", "QR image not found")
			return
		}
		png, err := qrcode.BWSPNG(account.MID)
		if err != nil {
			writeBusinessError(w, "qr_generate_failed", err.Error())
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(png)
		return
	}
	qrPath, err := h.deps.Store.UserQRPath(r.Context(), userID)
	if err != nil || qrPath == "" {
		writeBusinessError(w, "qr_not_found", "QR image not found")
		return
	}
	http.ServeFile(w, r, filepath.Join(h.deps.UploadDir, filepath.Base(qrPath)))
}

func (h Handler) bilibiliAccount(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	account, err := h.deps.Store.BilibiliAccount(r.Context(), user.ID)
	if errors.Is(err, sql.ErrNoRows) {
		writeOK(w, map[string]any{"bound": false})
		return
	}
	if err != nil {
		writeBusinessError(w, "bilibili_account_load_failed", err.Error())
		return
	}
	writeOK(w, map[string]any{
		"bound": true,
		"account": map[string]any{
			"mid":             account.MID,
			"uname":           account.Uname,
			"faceUrl":         account.FaceURL,
			"cookieExpiresAt": account.CookieExpiresAt,
			"lastValidatedAt": account.LastValidatedAt,
		},
	})
}

func (h Handler) createBilibiliLoginQRCode(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentUser(w, r); !ok {
		return
	}
	if h.deps.Bilibili == nil {
		writeBusinessError(w, "bilibili_login_disabled", "")
		return
	}
	qr, err := h.deps.Bilibili.CreateLoginQRCode(r.Context())
	if err != nil {
		writeBusinessError(w, "bilibili_qrcode_create_failed", err.Error())
		return
	}
	imageDataURL, err := qrcode.PNGDataURL(qr.URL, 320)
	if err != nil {
		writeBusinessError(w, "bilibili_qrcode_image_failed", err.Error())
		return
	}
	writeOK(w, map[string]any{
		"qrcode": map[string]any{
			"url":          qr.URL,
			"qrcodeKey":    qr.QRCodeKey,
			"expiresAt":    qr.ExpiresAt,
			"imageDataUrl": imageDataURL,
		},
	})
}

type bilibiliLoginPollRequest struct {
	QRCodeKey string `json:"qrcodeKey"`
}

func (h Handler) pollBilibiliLoginQRCode(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if h.deps.Bilibili == nil {
		writeBusinessError(w, "bilibili_login_disabled", "")
		return
	}
	if h.deps.BilibiliCookieSecret == "" {
		writeBusinessError(w, "bilibili_cookie_secret_required", "")
		return
	}
	var input bilibiliLoginPollRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeBusinessError(w, "invalid_json", "invalid JSON")
		return
	}
	if input.QRCodeKey == "" {
		writeBusinessError(w, "bilibili_qrcode_key_required", "")
		return
	}
	poll, err := h.deps.Bilibili.PollLoginQRCode(r.Context(), input.QRCodeKey)
	if err != nil {
		writeBusinessError(w, "bilibili_qrcode_poll_failed", err.Error())
		return
	}
	if poll.Status != bilibili.LoginStatusConfirmed {
		writeOK(w, map[string]any{"status": poll.Status, "message": poll.Message})
		return
	}
	nav, err := h.deps.Bilibili.Nav(r.Context(), poll.Cookies)
	if err != nil {
		writeBusinessError(w, "bilibili_nav_failed", err.Error())
		return
	}
	cookieCiphertext, err := bilibili.EncryptCookieJar(h.deps.BilibiliCookieSecret, poll.Cookies)
	if err != nil {
		writeBusinessError(w, "bilibili_cookie_encrypt_failed", err.Error())
		return
	}
	now := time.Now().UTC()
	if err := h.deps.Store.SaveBilibiliAccount(r.Context(), domain.BilibiliAccount{
		UserID:           user.ID,
		MID:              nav.MID,
		Uname:            nav.Uname,
		FaceURL:          nav.FaceURL,
		CookieCiphertext: cookieCiphertext,
		LastValidatedAt:  &now,
	}); err != nil {
		writeBusinessError(w, "bilibili_account_save_failed", err.Error())
		return
	}
	h.audit(r, user.ID, "bilibili.bind", "", user.ID, "")
	writeOK(w, map[string]any{
		"status": bilibili.LoginStatusConfirmed,
		"account": map[string]any{
			"mid":     nav.MID,
			"uname":   nav.Uname,
			"faceUrl": nav.FaceURL,
		},
	})
}

func (h Handler) unbindBilibiliAccount(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	if err := h.deps.Store.UnbindBilibiliAccount(r.Context(), user.ID); err != nil {
		writeBusinessError(w, "bilibili_account_unbind_failed", err.Error())
		return
	}
	if user.QRSource == domain.QRSourceBilibiliGenerated {
		_ = h.deps.Store.SetUserQRSource(r.Context(), user.ID, domain.QRSourceUploaded)
	}
	h.audit(r, user.ID, "bilibili.unbind", "", user.ID, "")
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
	if groupID == "" {
		writeBusinessError(w, "group_id_required", "")
		return
	}
	if err := h.deps.Store.JoinGroup(r.Context(), groupID, user.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeBusinessError(w, "group_not_found", "")
			return
		}
		if errors.Is(err, store.ErrGroupArchived) {
			writeBusinessError(w, "group_archived", "")
			return
		}
		if errors.Is(err, store.ErrGroupJoinLocked) {
			writeBusinessError(w, "group_join_locked", "")
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
	if h.deps.TaskSync != nil {
		h.deps.TaskSync.EnsureFresh(r.Context())
	}
	tasks, err := h.deps.Store.GroupTasks(r.Context(), groupID)
	if err != nil {
		writeBusinessError(w, "tasks_load_failed", err.Error())
		return
	}
	writeOK(w, map[string][]domain.TaskStatus{"tasks": tasks})
}

func (h Handler) syncGroupTasks(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	var input groupIDRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeBusinessError(w, "invalid_json", "invalid JSON")
		return
	}
	if input.GroupID == "" {
		writeBusinessError(w, "group_id_required", "")
		return
	}
	if !h.requireOwner(w, r, input.GroupID, user.ID) {
		return
	}
	if h.deps.TaskSync == nil {
		writeBusinessError(w, "task_sync_disabled", "")
		return
	}
	account, err := h.deps.Store.BilibiliAccount(r.Context(), user.ID)
	if errors.Is(err, sql.ErrNoRows) {
		writeBusinessError(w, "creator_bilibili_account_required", "")
		return
	}
	if err != nil {
		writeBusinessError(w, "bilibili_account_load_failed", err.Error())
		return
	}
	if err := h.deps.TaskSync.SyncWithAccount(r.Context(), account); err != nil {
		if errors.Is(err, tasksync.ErrEmptyTaskList) {
			writeBusinessError(w, "empty_task_list", "")
			return
		}
		writeBusinessError(w, "task_sync_failed", err.Error())
		return
	}
	state, err := h.deps.Store.TaskSyncState(r.Context())
	if err != nil {
		writeBusinessError(w, "task_sync_status_failed", err.Error())
		return
	}
	h.audit(r, user.ID, "task.sync", input.GroupID, "", "")
	writeOK(w, map[string]store.TaskSyncState{"sync": state})
}

func (h Handler) taskSyncStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentUser(w, r); !ok {
		return
	}
	state, err := h.deps.Store.TaskSyncState(r.Context())
	if err != nil {
		writeBusinessError(w, "task_sync_status_failed", err.Error())
		return
	}
	writeOK(w, map[string]store.TaskSyncState{"sync": state})
}

func (h Handler) syncTasks(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentUser(w, r); !ok {
		return
	}
	if h.deps.TaskSync == nil {
		writeBusinessError(w, "task_sync_disabled", "")
		return
	}
	if err := h.deps.TaskSync.Sync(r.Context()); err != nil {
		if errors.Is(err, tasksync.ErrEmptyTaskList) {
			writeBusinessError(w, "empty_task_list", "")
			return
		}
		writeBusinessError(w, "task_sync_failed", err.Error())
		return
	}
	state, err := h.deps.Store.TaskSyncState(r.Context())
	if err != nil {
		writeBusinessError(w, "task_sync_status_failed", err.Error())
		return
	}
	writeOK(w, map[string]store.TaskSyncState{"sync": state})
}

func (h Handler) taskImage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.currentUser(w, r); !ok {
		return
	}
	taskID := r.URL.Query().Get("taskId")
	if taskID == "" {
		writeBusinessError(w, "task_id_required", "")
		return
	}
	task, err := h.deps.Store.TaskByID(r.Context(), taskID)
	if err != nil || task.ImageURL == "" {
		writeBusinessError(w, "task_image_not_found", "")
		return
	}
	imageURL, ok := normalizeTaskImageURL(task.ImageURL)
	if !ok {
		writeBusinessError(w, "task_image_not_found", "")
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, imageURL, nil)
	if err != nil {
		writeBusinessError(w, "task_image_load_failed", err.Error())
		return
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/png,image/jpeg,image/*,*/*;q=0.8")
	req.Header.Set("Referer", "https://www.bilibili.com/blackboard/era/bws2026-live.html#/map")
	req.Header.Set("User-Agent", "BWS-Checkin/1.0")
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		writeBusinessError(w, "task_image_load_failed", err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeBusinessError(w, "task_image_load_failed", "")
		return
	}
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		writeBusinessError(w, "task_image_invalid", "")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 4<<20))
}

func normalizeTaskImageURL(rawURL string) (string, bool) {
	value := strings.TrimSpace(rawURL)
	if strings.HasPrefix(value, "//") {
		value = "https:" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", false
	}
	return parsed.String(), true
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
		if errors.Is(err, store.ErrLiveCompletionLocked) {
			writeBusinessError(w, "live_completion_locked", err.Error())
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
		if errors.Is(err, store.ErrLiveCompletionLocked) {
			writeBusinessError(w, "live_completion_locked", err.Error())
			return
		}
		writeBusinessError(w, "task_uncomplete_failed", err.Error())
		return
	}
	h.audit(r, user.ID, "task.uncomplete", groupID, input.UserID, input.TaskID)
	writeOK(w, map[string]bool{"ok": true})
}

func (h Handler) refreshTaskStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	var input taskCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeBusinessError(w, "invalid_json", "invalid JSON")
		return
	}
	if h.deps.Bilibili == nil {
		writeBusinessError(w, "bilibili_login_disabled", "")
		return
	}
	if h.deps.BilibiliCookieSecret == "" {
		writeBusinessError(w, "bilibili_cookie_secret_required", "")
		return
	}
	if _, err := h.deps.Store.GroupByID(r.Context(), input.GroupID, user.ID); err != nil {
		writeNotFoundOrForbidden(w, err)
		return
	}
	task, err := h.deps.Store.TaskByID(r.Context(), input.TaskID)
	if err != nil {
		writeBusinessError(w, "task_not_found", "")
		return
	}
	venueID, err := strconv.Atoi(task.VenueID)
	if err != nil || venueID == 0 || task.EventDay == "" || task.ExternalID == "" {
		writeBusinessError(w, "task_live_metadata_missing", "")
		return
	}
	account, err := h.deps.Store.BilibiliAccount(r.Context(), input.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		writeBusinessError(w, "bilibili_account_required", "")
		return
	}
	if err != nil {
		writeBusinessError(w, "bilibili_account_load_failed", err.Error())
		return
	}
	cookies, err := bilibili.DecryptCookieJar(h.deps.BilibiliCookieSecret, account.CookieCiphertext)
	if err != nil {
		writeBusinessError(w, "bilibili_cookie_decrypt_failed", err.Error())
		return
	}
	points, err := h.deps.Bilibili.OfflinePoints(r.Context(), bilibili.OfflinePointsRequest{
		BID:     202601,
		Year:    202601,
		VenueID: venueID,
		Day:     task.EventDay,
	}, cookies)
	if err != nil {
		_ = h.deps.Store.MarkLiveTaskCompletionStale(r.Context(), input.GroupID, input.TaskID, input.UserID, time.Now().UTC())
		writeBusinessError(w, "task_status_refresh_failed", err.Error())
		return
	}
	completed := false
	for _, point := range points {
		if point.ID == task.ExternalID {
			completed = point.Completed
			break
		}
	}
	status := domain.CompletionStatusLiveIncomplete
	if completed {
		status = domain.CompletionStatusLiveCompleted
	}
	now := time.Now().UTC()
	if err := h.deps.Store.UpsertLiveTaskCompletion(r.Context(), store.LiveTaskCompletionInput{
		GroupID:       input.GroupID,
		TaskID:        input.TaskID,
		TargetUserID:  input.UserID,
		Status:        status,
		LiveCheckedAt: now,
		UpdatedAt:     now,
	}); err != nil {
		writeBusinessError(w, "task_status_save_failed", err.Error())
		return
	}
	h.audit(r, user.ID, "task.status_refresh", input.GroupID, input.UserID, input.TaskID)
	writeOK(w, map[string]string{"status": status})
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
		writeBusinessError(w, "group_access_denied", "")
		return
	}
	writeBusinessError(w, "request_failed", err.Error())
}

func businessErrorMessage(code string, fallback string) string {
	messages := map[string]string{
		"current_user_load_failed":          "当前用户加载失败，请重新登录",
		"creator_bilibili_account_required": "创建者需要先在个人中心完成 B 站扫码登录",
		"empty_task_list":                   "未同步到任何乐园任务，请检查 B 站登录状态或稍后重试",
		"file_required":                     "请选择二维码图片",
		"group_access_denied":               "互助组不存在或你无权访问",
		"group_archived":                    "互助组已归档，不能继续操作",
		"group_id_conflict":                 "互助组 ID 已存在或输入不合法",
		"group_id_required":                 "请输入互助组 ID",
		"group_join_failed":                 "加入互助组失败，请稍后重试",
		"group_join_locked":                 "互助组已锁定，暂时不能加入",
		"group_not_found":                   "互助组不存在",
		"invalid_group_input":               "请填写完整的互助组信息",
		"invalid_image":                     "二维码图片无法识别",
		"invalid_json":                      "请求格式不正确",
		"invalid_upload":                    "上传内容无效",
		"login_required":                    "请先登录",
		"owner_role_required":               "只有创建者可以执行此操作",
		"qr_delete_failed":                  "删除二维码失败，请稍后重试",
		"qr_not_found":                      "二维码图片不存在",
		"qr_save_failed":                    "保存二维码失败，请稍后重试",
		"qr_update_failed":                  "更新二维码失败，请稍后重试",
		"task_id_required":                  "缺少任务 ID",
		"task_image_invalid":                "点位图片无法识别",
		"task_image_load_failed":            "点位图片加载失败",
		"task_image_not_found":              "点位图片不存在",
		"unsupported_image_type":            "只支持 PNG、JPG、JPEG 或 WebP 图片",
		"user_id_required":                  "缺少用户 ID",
	}
	if message, ok := messages[code]; ok {
		return message
	}
	if fallback == "" || looksLikeDatabaseError(fallback) {
		return "操作失败，请稍后重试"
	}
	return fallback
}

func looksLikeDatabaseError(message string) bool {
	lower := strings.ToLower(message)
	patterns := []string{
		"sql:",
		"sql logic error",
		"constraint failed",
		"unique constraint",
		"foreign key constraint",
		"check constraint",
		"database is locked",
		"no such table",
		"no column named",
		"rows in result set",
	}
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
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
