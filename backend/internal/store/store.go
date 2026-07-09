package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bws-checkin/backend/internal/domain"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

type User = domain.User

type Store struct {
	db *sql.DB
}

var (
	ErrGroupJoinLocked      = errors.New("group join locked")
	ErrGroupArchived        = errors.New("group archived")
	ErrLiveCompletionLocked = errors.New("live completion locked")
)

type CreateGroupInput struct {
	ID          string
	Name        string
	Day         string
	Description string
	OwnerUserID string
}

type UpdateGroupInput struct {
	ID          string
	Name        string
	Day         string
	Description string
}

type AuditLogInput struct {
	ActorUserID  string
	Action       string
	GroupID      string
	TargetUserID string
	TaskID       string
	Metadata     string
}

type AuditLog struct {
	ID           int64
	ActorUserID  string
	Action       string
	GroupID      string
	TargetUserID string
	TaskID       string
	Metadata     string
	CreatedAt    string
}

type SyncedTaskInput struct {
	ID          string
	ExternalID  string
	GroupName   string
	Name        string
	Title       string
	RewardCoins int
	Description string
	ImageURL    string
	VenueID     string
	VenueName   string
	EventDay    string
	SortOrder   int
}

type TaskSyncState struct {
	LastSuccessAt *time.Time
	LastErrorAt   *time.Time
	LastErrorCode string
	UpdatedAt     *time.Time
}

type SyncTaskCompletionInput struct {
	GroupID         string
	TaskID          string
	TargetUserID    string
	CheckedByUserID string
	Completed       bool
	UpdatedAt       time.Time
}

type LiveTaskCompletionInput struct {
	GroupID       string
	TaskID        string
	TargetUserID  string
	Status        string
	LiveCheckedAt time.Time
	UpdatedAt     time.Time
	LiveStale     bool
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func OpenMemory() (*Store, error) {
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	body, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(string(body)); err != nil {
		return err
	}
	if err := s.ensureColumn("users", "qr_source", "TEXT NOT NULL DEFAULT 'uploaded'"); err != nil {
		return err
	}
	if err := s.ensureColumn("tasks", "external_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("tasks", "group_name", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("tasks", "title", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("tasks", "reward_coins", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn("tasks", "description", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("tasks", "image_url", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("tasks", "venue_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("tasks", "venue_name", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("tasks", "event_day", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("tasks", "sync_source", "TEXT NOT NULL DEFAULT 'default'"); err != nil {
		return err
	}
	if err := s.ensureTaskCompletionColumn("completed", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	if err := s.ensureTaskCompletionColumn("status", "TEXT NOT NULL DEFAULT 'manual_completed'"); err != nil {
		return err
	}
	if err := s.ensureTaskCompletionColumn("source", "TEXT NOT NULL DEFAULT 'manual'"); err != nil {
		return err
	}
	if err := s.ensureTaskCompletionColumn("live_checked_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureTaskCompletionColumn("live_stale", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureTaskCompletionColumn("updated_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn("groups", "join_locked", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn("groups", "archived_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureGroupDayConstraint(); err != nil {
		return err
	}
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS bilibili_accounts (
			user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			mid TEXT NOT NULL,
			uname TEXT NOT NULL,
			face_url TEXT NOT NULL DEFAULT '',
			cookie_ciphertext TEXT NOT NULL,
			cookie_expires_at TEXT NOT NULL DEFAULT '',
			refresh_token_ciphertext TEXT NOT NULL DEFAULT '',
			last_validated_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS task_sync_state (
			id TEXT PRIMARY KEY,
			last_success_at TEXT NOT NULL DEFAULT '',
			last_error_at TEXT NOT NULL DEFAULT '',
			last_error_code TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			actor_user_id TEXT NOT NULL REFERENCES users(id),
			action TEXT NOT NULL,
			group_id TEXT NOT NULL DEFAULT '',
			target_user_id TEXT,
			task_id TEXT NOT NULL DEFAULT '',
			metadata TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return err
	}
	if err := s.seedDefaultTasks(); err != nil {
		return err
	}
	_, err = s.db.Exec(`
		UPDATE task_completions
		SET
			updated_at = CASE WHEN updated_at = '' THEN completed_at ELSE updated_at END,
			status = CASE
				WHEN source = 'live' AND completed = 1 THEN 'live_completed'
				WHEN source = 'live' AND completed = 0 THEN 'live_incomplete'
				WHEN completed = 1 THEN 'manual_completed'
				ELSE 'manual_incomplete'
			END,
			source = CASE WHEN status IN ('live_incomplete', 'live_completed') THEN 'live' ELSE source END
	`)
	return err
}

func (s *Store) seedDefaultTasks() error {
	_, err := s.db.Exec(`
		INSERT INTO tasks (
			id, external_id, group_name, name, title, reward_coins, description,
			image_url, venue_id, venue_name, event_day, sync_source, sort_order, enabled
		)
		VALUES
			('rainbow-station', '', '8.1馆', '彩虹补给站', '完成彩虹补给站互动', 3, '在彩虹补给站完成互动并出示二维码。', '', '1', '8.1馆', '20260710', 'default', 10, 1),
			('stage-support', '', '8.1馆', '舞台应援任务', '完成主舞台应援', 5, '在主舞台完成应援任务并领取奖励。', '', '1', '8.1馆', '20260710', 'default', 20, 1),
			('stamp-rally', '', '1.1馆', '乐园集章点', '完成乐园集章点打卡', 2, '到达集章点完成盖章或扫码确认。', '', '2', '1.1馆', '20260710', 'default', 30, 1),
			('photo-spot', '', '3馆', '主题合影点', '完成主题合影点互动', 2, '在主题合影点完成互动拍照后出示二维码。', '', '4', '3馆', '20260710', 'default', 40, 1),
			('default-20260711-supply', '', '1.1馆', '乐园补给站', '领取乐园补给奖励', 3, '在 1.1 馆补给点完成互动并出示二维码。', '', '2', '1.1馆', '20260711', 'default', 110, 1),
			('default-20260711-virtual', '', '虚拟乐园', '虚拟乐园应援', '完成虚拟乐园互动', 5, '在虚拟乐园完成指定互动并领取乐园币。', '', '6', '6.1馆', '20260711', 'default', 120, 1),
			('default-20260711-market', '', '梦幻集市', '梦幻集市集章', '完成梦幻集市集章', 2, '前往梦幻集市完成集章或扫码确认。', '', '5', '4.1馆', '20260711', 'default', 130, 1),
			('default-20260711-boardgame', '', '一起桌游', '桌游试玩挑战', '完成桌游试玩挑战', 2, '在桌游区域完成试玩互动后出示二维码。', '', '7', '5.1馆', '20260711', 'default', 140, 1),
			('default-20260712-parade', '', '3馆', '主题巡游点', '完成主题巡游互动', 3, '在主题巡游点完成互动并出示二维码。', '', '4', '3馆', '20260712', 'default', 210, 1),
			('default-20260712-heart', '', '恋恋心声', '恋恋心声互动', '完成恋恋心声互动', 5, '在恋恋心声区域完成指定互动。', '', '5', '4.1馆', '20260712', 'default', 220, 1),
			('default-20260712-game', '', '游戏世界', '游戏世界挑战', '完成游戏世界挑战', 2, '前往游戏世界完成试玩或扫码确认。', '', '3', '2.1馆', '20260712', 'default', 230, 1),
			('default-20260712-pain', '', '痛无止境', '痛无止境集章', '完成痛无止境集章', 2, '在痛无止境区域完成集章互动。', '', '7', '5.1馆', '20260712', 'default', 240, 1)
		ON CONFLICT(id) DO UPDATE SET
			external_id = excluded.external_id,
			group_name = excluded.group_name,
			name = excluded.name,
			title = excluded.title,
			reward_coins = excluded.reward_coins,
			description = excluded.description,
			image_url = excluded.image_url,
			venue_id = excluded.venue_id,
			venue_name = excluded.venue_name,
			event_day = excluded.event_day,
			sort_order = excluded.sort_order,
			enabled = 1
		WHERE tasks.sync_source = 'default'
	`)
	return err
}

func (s *Store) ensureTaskCompletionColumn(name string, definition string) error {
	return s.ensureColumn("task_completions", name, definition)
}

func (s *Store) ensureColumn(table string, name string, definition string) error {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var columnName string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return err
		}
		if columnName == name {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + name + ` ` + definition)
	return err
}

func (s *Store) ensureGroupDayConstraint() error {
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	var tableSQL string
	if err := conn.QueryRowContext(ctx, `
		SELECT sql
		FROM sqlite_master
		WHERE type = 'table' AND name = 'groups'
	`).Scan(&tableSQL); err != nil {
		return err
	}
	if strings.Contains(tableSQL, "'20260710'") &&
		strings.Contains(tableSQL, "'20260711'") &&
		strings.Contains(tableSQL, "'20260712'") {
		return nil
	}
	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer func() { _, _ = conn.ExecContext(ctx, `PRAGMA foreign_keys = ON`) }()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`
		CREATE TABLE groups_new (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			day TEXT NOT NULL CHECK (day IN ('20260710', '20260711', '20260712', 'friday', 'saturday', 'sunday')),
			description TEXT NOT NULL DEFAULT '',
			owner_user_id TEXT NOT NULL REFERENCES users(id),
			join_locked INTEGER NOT NULL DEFAULT 0,
			archived_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO groups_new (
			id, name, day, description, owner_user_id, join_locked, archived_at, created_at, updated_at
		)
		SELECT id, name, day, description, owner_user_id, join_locked, archived_at, created_at, updated_at
		FROM groups
	`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE groups`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE groups_new RENAME TO groups`); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	rows, err := conn.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		var table string
		var rowID int64
		var parent string
		var fkID int
		if err := rows.Scan(&table, &rowID, &parent, &fkID); err != nil {
			return err
		}
		return fmt.Errorf("foreign key violation after groups migration: table=%s rowid=%d parent=%s fkid=%d", table, rowID, parent, fkID)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}

func (s *Store) UpsertUser(ctx context.Context, subject, displayName string) (domain.User, error) {
	if subject == "" || displayName == "" {
		return domain.User{}, errors.New("subject and display name are required")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (id, oidc_subject, display_name)
		VALUES (?, ?, ?)
		ON CONFLICT(oidc_subject) DO UPDATE SET display_name = excluded.display_name, updated_at = CURRENT_TIMESTAMP
	`, newUUID(), subject, displayName)
	if err != nil {
		return domain.User{}, err
	}
	return s.UserBySubject(ctx, subject)
}

func (s *Store) UserBySubject(ctx context.Context, subject string) (domain.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `
		SELECT id, display_name, avatar_url, qr_image_path, qr_source
		FROM users
		WHERE oidc_subject = ?
	`, subject))
}

func (s *Store) UserByID(ctx context.Context, id string) (domain.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `
		SELECT id, display_name, avatar_url, qr_image_path, qr_source
		FROM users
		WHERE id = ?
	`, id))
}

func (s *Store) UpdateUserQR(ctx context.Context, userID string, path string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET qr_image_path = ?, qr_source = 'uploaded', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, path, userID)
	return err
}

func (s *Store) SetUserQRSource(ctx context.Context, userID string, source string) error {
	if source != domain.QRSourceUploaded && source != domain.QRSourceBilibiliGenerated {
		return fmt.Errorf("unsupported qr source: %s", source)
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET qr_source = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, source, userID)
	return err
}

func (s *Store) UserQRPath(ctx context.Context, userID string) (string, error) {
	var path string
	err := s.db.QueryRowContext(ctx, `
		SELECT qr_image_path
		FROM users
		WHERE id = ?
	`, userID).Scan(&path)
	return path, err
}

func (s *Store) SaveBilibiliAccount(ctx context.Context, account domain.BilibiliAccount) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO bilibili_accounts (
			user_id, mid, uname, face_url, cookie_ciphertext, cookie_expires_at,
			refresh_token_ciphertext, last_validated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			mid = excluded.mid,
			uname = excluded.uname,
			face_url = excluded.face_url,
			cookie_ciphertext = excluded.cookie_ciphertext,
			cookie_expires_at = excluded.cookie_expires_at,
			refresh_token_ciphertext = excluded.refresh_token_ciphertext,
			last_validated_at = excluded.last_validated_at,
			updated_at = CURRENT_TIMESTAMP
	`, account.UserID, account.MID, account.Uname, account.FaceURL, account.CookieCiphertext, formatOptionalTime(account.CookieExpiresAt), account.RefreshTokenCiphertext, formatOptionalTime(account.LastValidatedAt))
	return err
}

func (s *Store) BilibiliAccount(ctx context.Context, userID string) (domain.BilibiliAccount, error) {
	var account domain.BilibiliAccount
	var cookieExpiresAt string
	var lastValidatedAt string
	var createdAt string
	var updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, mid, uname, face_url, cookie_ciphertext, cookie_expires_at,
			refresh_token_ciphertext, last_validated_at, created_at, updated_at
		FROM bilibili_accounts
		WHERE user_id = ?
	`, userID).Scan(
		&account.UserID,
		&account.MID,
		&account.Uname,
		&account.FaceURL,
		&account.CookieCiphertext,
		&cookieExpiresAt,
		&account.RefreshTokenCiphertext,
		&lastValidatedAt,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return domain.BilibiliAccount{}, err
	}
	account.CookieExpiresAt = parseOptionalTime(cookieExpiresAt)
	account.LastValidatedAt = parseOptionalTime(lastValidatedAt)
	account.CreatedAt = parseOptionalTime(createdAt)
	account.UpdatedAt = parseOptionalTime(updatedAt)
	return account, nil
}

func (s *Store) AnyBilibiliAccount(ctx context.Context) (domain.BilibiliAccount, error) {
	var userID string
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id
		FROM bilibili_accounts
		ORDER BY last_validated_at DESC, updated_at DESC
		LIMIT 1
	`).Scan(&userID)
	if err != nil {
		return domain.BilibiliAccount{}, err
	}
	return s.BilibiliAccount(ctx, userID)
}

func (s *Store) UnbindBilibiliAccount(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM bilibili_accounts
		WHERE user_id = ?
	`, userID)
	return err
}

func (s *Store) CreateGroup(ctx context.Context, input CreateGroupInput) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO groups (id, name, day, description, owner_user_id)
		VALUES (?, ?, ?, ?, ?)
	`, input.ID, input.Name, input.Day, input.Description, input.OwnerUserID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO group_members (group_id, user_id, role)
		VALUES (?, ?, 'owner')
	`, input.ID, input.OwnerUserID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) JoinGroup(ctx context.Context, groupID string, userID string) error {
	joinLocked, archived, err := s.groupFlags(ctx, groupID)
	if err != nil {
		return err
	}
	if archived {
		return ErrGroupArchived
	}
	if joinLocked {
		return ErrGroupJoinLocked
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO group_members (group_id, user_id, role)
		VALUES (?, ?, 'member')
	`, groupID, userID)
	return err
}

func (s *Store) UserGroups(ctx context.Context, userID string, includeArchived bool) ([]domain.Group, error) {
	archiveFilter := "AND g.archived_at = ''"
	if includeArchived {
		archiveFilter = ""
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.id, g.name, g.day, g.description, gm.role, g.join_locked, g.archived_at,
			(SELECT COUNT(*) FROM group_members WHERE group_id = g.id) AS member_count,
			(SELECT COUNT(*) FROM tasks WHERE enabled = 1) AS task_count
		FROM groups g
		JOIN group_members gm ON gm.group_id = g.id
		WHERE gm.user_id = ? `+archiveFilter+`
		ORDER BY g.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []domain.Group
	for rows.Next() {
		var group domain.Group
		var joinLocked int
		var archivedAt string
		if err := rows.Scan(
			&group.ID,
			&group.Name,
			&group.Day,
			&group.Description,
			&group.Role,
			&joinLocked,
			&archivedAt,
			&group.MemberCount,
			&group.TaskCount,
		); err != nil {
			return nil, err
		}
		group.JoinLocked = joinLocked == 1
		group.ArchivedAt = parseOptionalTime(archivedAt)
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (s *Store) GroupByID(ctx context.Context, groupID string, userID string) (domain.Group, error) {
	var group domain.Group
	var joinLocked int
	var archivedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT g.id, g.name, g.day, g.description, gm.role, g.join_locked, g.archived_at,
			(SELECT COUNT(*) FROM group_members WHERE group_id = g.id) AS member_count,
			(SELECT COUNT(*) FROM tasks WHERE enabled = 1) AS task_count
		FROM groups g
		JOIN group_members gm ON gm.group_id = g.id
		WHERE g.id = ? AND gm.user_id = ?
	`, groupID, userID).Scan(
		&group.ID,
		&group.Name,
		&group.Day,
		&group.Description,
		&group.Role,
		&joinLocked,
		&archivedAt,
		&group.MemberCount,
		&group.TaskCount,
	)
	group.JoinLocked = joinLocked == 1
	group.ArchivedAt = parseOptionalTime(archivedAt)
	return group, err
}

func (s *Store) UpdateGroup(ctx context.Context, input UpdateGroupInput) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE groups
		SET name = ?, day = ?, description = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, input.Name, input.Day, input.Description, input.ID)
	return err
}

func (s *Store) SetGroupJoinLocked(ctx context.Context, groupID string, locked bool) error {
	value := 0
	if locked {
		value = 1
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE groups
		SET join_locked = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, value, groupID)
	return err
}

func (s *Store) ArchiveGroup(ctx context.Context, groupID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE groups
		SET archived_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND archived_at = ''
	`, time.Now().UTC().Format(time.RFC3339Nano), groupID)
	return err
}

func (s *Store) AppendAuditLog(ctx context.Context, input AuditLogInput) error {
	metadata := input.Metadata
	if metadata == "" {
		metadata = "{}"
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_logs (actor_user_id, action, group_id, target_user_id, task_id, metadata)
		VALUES (?, ?, ?, ?, ?, ?)
	`, input.ActorUserID, input.Action, input.GroupID, nullString(input.TargetUserID), input.TaskID, metadata)
	return err
}

func (s *Store) AuditLogs(ctx context.Context) ([]AuditLog, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, actor_user_id, action, group_id, COALESCE(target_user_id, ''), task_id, metadata, created_at
		FROM audit_logs
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []AuditLog
	for rows.Next() {
		var log AuditLog
		if err := rows.Scan(
			&log.ID,
			&log.ActorUserID,
			&log.Action,
			&log.GroupID,
			&log.TargetUserID,
			&log.TaskID,
			&log.Metadata,
			&log.CreatedAt,
		); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

func (s *Store) IsOwner(ctx context.Context, groupID string, userID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM group_members
		WHERE group_id = ? AND user_id = ? AND role = 'owner'
	`, groupID, userID).Scan(&count)
	return count > 0, err
}

func (s *Store) RemoveMember(ctx context.Context, groupID string, userID string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM group_members
		WHERE group_id = ? AND user_id = ? AND role != 'owner'
	`, groupID, userID)
	return err
}

func (s *Store) ReplaceBilibiliTasks(ctx context.Context, tasks []SyncedTaskInput) error {
	if len(tasks) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE tasks
		SET enabled = 0
		WHERE sync_source IN ('default', 'bilibili')
	`); err != nil {
		return err
	}
	for _, task := range tasks {
		if task.ID == "" || task.Name == "" {
			continue
		}
		if task.Title == "" {
			task.Title = task.Name
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO tasks (
				id, external_id, group_name, name, title, reward_coins, description,
				image_url, venue_id, venue_name, event_day, sync_source, sort_order, enabled
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'bilibili', ?, 1)
			ON CONFLICT(id) DO UPDATE SET
				external_id = excluded.external_id,
				group_name = excluded.group_name,
				name = excluded.name,
				title = excluded.title,
				reward_coins = excluded.reward_coins,
				description = excluded.description,
				image_url = excluded.image_url,
				venue_id = excluded.venue_id,
				venue_name = excluded.venue_name,
				event_day = excluded.event_day,
				sync_source = excluded.sync_source,
				sort_order = excluded.sort_order,
				enabled = 1
		`, task.ID, task.ExternalID, task.GroupName, task.Name, task.Title, task.RewardCoins, task.Description, task.ImageURL, task.VenueID, task.VenueName, task.EventDay, task.SortOrder)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) RecordTaskSyncSuccess(ctx context.Context, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO task_sync_state (id, last_success_at, last_error_at, last_error_code, updated_at)
		VALUES ('bws_tasks', ?, '', '', ?)
		ON CONFLICT(id) DO UPDATE SET
			last_success_at = excluded.last_success_at,
			last_error_at = '',
			last_error_code = '',
			updated_at = excluded.updated_at
	`, at.UTC().Format(time.RFC3339Nano), at.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) RecordTaskSyncError(ctx context.Context, code string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO task_sync_state (id, last_success_at, last_error_at, last_error_code, updated_at)
		VALUES ('bws_tasks', '', ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			last_error_at = excluded.last_error_at,
			last_error_code = excluded.last_error_code,
			updated_at = excluded.updated_at
	`, at.UTC().Format(time.RFC3339Nano), code, at.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) TaskSyncState(ctx context.Context) (TaskSyncState, error) {
	var lastSuccessAt string
	var lastErrorAt string
	var state TaskSyncState
	var updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT last_success_at, last_error_at, last_error_code, updated_at
		FROM task_sync_state
		WHERE id = 'bws_tasks'
	`).Scan(&lastSuccessAt, &lastErrorAt, &state.LastErrorCode, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return TaskSyncState{}, nil
	}
	if err != nil {
		return TaskSyncState{}, err
	}
	state.LastSuccessAt = parseOptionalTime(lastSuccessAt)
	state.LastErrorAt = parseOptionalTime(lastErrorAt)
	state.UpdatedAt = parseOptionalTime(updatedAt)
	return state, nil
}

func (s *Store) GroupTasks(ctx context.Context, groupID string) ([]domain.TaskStatus, error) {
	eventDay, err := s.groupEventDay(ctx, groupID)
	if err != nil {
		return nil, err
	}
	members, err := s.groupMembers(ctx, groupID)
	if err != nil {
		return nil, err
	}
	tasks, err := s.enabledTasks(ctx, eventDay)
	if err != nil {
		return nil, err
	}
	completions, err := s.completions(ctx, groupID)
	if err != nil {
		return nil, err
	}

	for taskIndex := range tasks {
		tasks[taskIndex].TotalCount = len(members)
		for _, member := range members {
			item := domain.MemberCompletion{
				Member:     member,
				Completed:  false,
				Status:     domain.CompletionStatusManualIncomplete,
				Source:     domain.CompletionSourceManual,
				CanToggle:  true,
				CanRefresh: false,
			}
			if completion, ok := completions[completionKey{TaskID: tasks[taskIndex].ID, UserID: member.ID}]; ok {
				item = completion
				item.Member = member
				if item.Completed {
					tasks[taskIndex].CompletedCount++
				}
			}
			tasks[taskIndex].Members = append(tasks[taskIndex].Members, item)
		}
	}
	return tasks, nil
}

func (s *Store) groupEventDay(ctx context.Context, groupID string) (string, error) {
	var day string
	if err := s.db.QueryRowContext(ctx, `SELECT day FROM groups WHERE id = ?`, groupID).Scan(&day); err != nil {
		return "", err
	}
	return normalizeEventDay(day), nil
}

func (s *Store) TaskByID(ctx context.Context, taskID string) (domain.TaskStatus, error) {
	var task domain.TaskStatus
	err := s.db.QueryRowContext(ctx, `
		SELECT id, external_id, group_name, name, title, reward_coins, description,
			image_url, venue_id, venue_name, event_day, sync_source, sort_order
		FROM tasks
		WHERE id = ?
	`, taskID).Scan(
		&task.ID,
		&task.ExternalID,
		&task.GroupName,
		&task.Name,
		&task.Title,
		&task.RewardCoins,
		&task.Description,
		&task.ImageURL,
		&task.VenueID,
		&task.VenueName,
		&task.EventDay,
		&task.SyncSource,
		&task.SortOrder,
	)
	return task, err
}

func (s *Store) MarkComplete(ctx context.Context, groupID string, taskID string, targetUserID string, checkedByUserID string) error {
	return s.SyncTaskCompletion(ctx, SyncTaskCompletionInput{
		GroupID:         groupID,
		TaskID:          taskID,
		TargetUserID:    targetUserID,
		CheckedByUserID: checkedByUserID,
		Completed:       true,
		UpdatedAt:       time.Now().UTC(),
	})
}

func (s *Store) UnmarkComplete(ctx context.Context, groupID string, taskID string, targetUserID string) error {
	return s.SyncTaskCompletion(ctx, SyncTaskCompletionInput{
		GroupID:         groupID,
		TaskID:          taskID,
		TargetUserID:    targetUserID,
		CheckedByUserID: targetUserID,
		Completed:       false,
		UpdatedAt:       time.Now().UTC(),
	})
}

func (s *Store) SyncTaskCompletion(ctx context.Context, input SyncTaskCompletionInput) error {
	if _, archived, err := s.groupFlags(ctx, input.GroupID); err != nil {
		return err
	} else if archived {
		return ErrGroupArchived
	}
	if locked, err := s.completionLiveLocked(ctx, input.GroupID, input.TaskID, input.TargetUserID); err != nil {
		return err
	} else if locked {
		return ErrLiveCompletionLocked
	}
	updatedAt := input.UpdatedAt.UTC()
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	timeValue := updatedAt.Format(time.RFC3339Nano)
	completed := 0
	status := domain.CompletionStatusManualIncomplete
	if input.Completed {
		completed = 1
		status = domain.CompletionStatusManualCompleted
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO task_completions (
			group_id, task_id, target_user_id, checked_by_user_id, completed, status, source,
			completed_at, live_checked_at, live_stale, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, 'manual', ?, '', 0, ?)
		ON CONFLICT(group_id, task_id, target_user_id) DO UPDATE SET
			checked_by_user_id = excluded.checked_by_user_id,
			completed = excluded.completed,
			status = excluded.status,
			source = excluded.source,
			completed_at = excluded.completed_at,
			live_checked_at = excluded.live_checked_at,
			live_stale = excluded.live_stale,
			updated_at = excluded.updated_at
		WHERE excluded.updated_at >= task_completions.updated_at
	`, input.GroupID, input.TaskID, input.TargetUserID, input.CheckedByUserID, completed, status, timeValue, timeValue)
	return err
}

func (s *Store) UpsertLiveTaskCompletion(ctx context.Context, input LiveTaskCompletionInput) error {
	if _, archived, err := s.groupFlags(ctx, input.GroupID); err != nil {
		return err
	} else if archived {
		return ErrGroupArchived
	}
	updatedAt := input.UpdatedAt.UTC()
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	liveCheckedAt := input.LiveCheckedAt.UTC()
	if liveCheckedAt.IsZero() {
		liveCheckedAt = updatedAt
	}
	status := input.Status
	if status == "" {
		status = domain.CompletionStatusLiveIncomplete
	}
	if status != domain.CompletionStatusLiveIncomplete && status != domain.CompletionStatusLiveCompleted {
		return fmt.Errorf("unsupported live completion status: %s", status)
	}
	completed := 0
	if status == domain.CompletionStatusLiveCompleted {
		completed = 1
	}
	liveStale := 0
	if input.LiveStale {
		liveStale = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO task_completions (
			group_id, task_id, target_user_id, checked_by_user_id, completed, status, source,
			completed_at, live_checked_at, live_stale, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, 'live', ?, ?, ?, ?)
		ON CONFLICT(group_id, task_id, target_user_id) DO UPDATE SET
			checked_by_user_id = excluded.checked_by_user_id,
			completed = excluded.completed,
			status = excluded.status,
			source = excluded.source,
			completed_at = excluded.completed_at,
			live_checked_at = excluded.live_checked_at,
			live_stale = excluded.live_stale,
			updated_at = excluded.updated_at
		WHERE task_completions.source = 'manual' OR excluded.updated_at >= task_completions.updated_at
	`, input.GroupID, input.TaskID, input.TargetUserID, input.TargetUserID, completed, status, updatedAt.Format(time.RFC3339Nano), liveCheckedAt.Format(time.RFC3339Nano), liveStale, updatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) MarkLiveTaskCompletionStale(ctx context.Context, groupID string, taskID string, targetUserID string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE task_completions
		SET live_stale = 1, updated_at = ?
		WHERE group_id = ? AND task_id = ? AND target_user_id = ? AND source = 'live'
	`, at.UTC().Format(time.RFC3339Nano), groupID, taskID, targetUserID)
	return err
}

func (s *Store) completionLiveLocked(ctx context.Context, groupID string, taskID string, targetUserID string) (bool, error) {
	var source string
	err := s.db.QueryRowContext(ctx, `
		SELECT source
		FROM task_completions
		WHERE group_id = ? AND task_id = ? AND target_user_id = ?
	`, groupID, taskID, targetUserID).Scan(&source)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return source == domain.CompletionSourceLive, nil
}

func scanUser(row *sql.Row) (domain.User, error) {
	var user domain.User
	var qrPath string
	if err := row.Scan(&user.ID, &user.DisplayName, &user.AvatarURL, &qrPath, &user.QRSource); err != nil {
		return domain.User{}, err
	}
	user.QRImageURL = qrAPIURL(user.ID, qrPath, user.QRSource)
	return user, nil
}

func qrAPIURL(userID string, qrPath string, qrSource string) string {
	if qrPath == "" && qrSource != domain.QRSourceBilibiliGenerated {
		return ""
	}
	return "/api/v1/user/qr?userId=" + userID
}

func (s *Store) groupFlags(ctx context.Context, groupID string) (bool, bool, error) {
	var joinLocked int
	var archivedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT join_locked, archived_at
		FROM groups
		WHERE id = ?
	`, groupID).Scan(&joinLocked, &archivedAt)
	if err != nil {
		return false, false, err
	}
	return joinLocked == 1, archivedAt != "", nil
}

func (s *Store) groupMembers(ctx context.Context, groupID string) ([]domain.Member, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.display_name, u.qr_image_path, u.qr_source
		FROM users u
		JOIN group_members gm ON gm.user_id = u.id
		WHERE gm.group_id = ?
		ORDER BY gm.joined_at ASC, u.id ASC
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []domain.Member
	for rows.Next() {
		var member domain.Member
		var qrPath string
		var qrSource string
		if err := rows.Scan(&member.ID, &member.DisplayName, &qrPath, &qrSource); err != nil {
			return nil, err
		}
		member.QRImageURL = qrAPIURL(member.ID, qrPath, qrSource)
		members = append(members, member)
	}
	return members, rows.Err()
}

func (s *Store) enabledTasks(ctx context.Context, eventDay string) ([]domain.TaskStatus, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, external_id, group_name, name, title, reward_coins, description,
			image_url, venue_id, venue_name, event_day, sync_source, sort_order
		FROM tasks
		WHERE enabled = 1 AND event_day = ?
		ORDER BY sort_order ASC, id ASC
	`, eventDay)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []domain.TaskStatus
	for rows.Next() {
		var task domain.TaskStatus
		if err := rows.Scan(
			&task.ID,
			&task.ExternalID,
			&task.GroupName,
			&task.Name,
			&task.Title,
			&task.RewardCoins,
			&task.Description,
			&task.ImageURL,
			&task.VenueID,
			&task.VenueName,
			&task.EventDay,
			&task.SyncSource,
			&task.SortOrder,
		); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func normalizeEventDay(day string) string {
	switch day {
	case "friday":
		return "20260710"
	case "saturday":
		return "20260711"
	case "sunday":
		return "20260712"
	default:
		return day
	}
}

type completionKey struct {
	TaskID string
	UserID string
}

func (s *Store) completions(ctx context.Context, groupID string) (map[completionKey]domain.MemberCompletion, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tc.task_id, tc.target_user_id, tc.checked_by_user_id, u.display_name,
			tc.completed, tc.status, tc.source, tc.completed_at, tc.live_checked_at,
			tc.live_stale, tc.updated_at
		FROM task_completions tc
		JOIN users u ON u.id = tc.checked_by_user_id
		WHERE tc.group_id = ?
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[completionKey]domain.MemberCompletion)
	for rows.Next() {
		var taskID string
		var targetUserID string
		var checkedByID string
		var checkedByName string
		var completed int
		var status string
		var source string
		var completedAtRaw string
		var liveCheckedAtRaw string
		var liveStale int
		var updatedAtRaw string
		if err := rows.Scan(&taskID, &targetUserID, &checkedByID, &checkedByName, &completed, &status, &source, &completedAtRaw, &liveCheckedAtRaw, &liveStale, &updatedAtRaw); err != nil {
			return nil, err
		}
		if status == "" {
			status = completionStatusFromCompleted(source, completed == 1)
		}
		if source == "" {
			source = domain.CompletionSourceManual
		}
		completedAt := parseSQLiteTime(completedAtRaw)
		canToggle := source != domain.CompletionSourceLive
		result[completionKey{TaskID: taskID, UserID: targetUserID}] = domain.MemberCompletion{
			Completed:     completed == 1,
			Status:        status,
			Source:        source,
			LiveStale:     liveStale == 1,
			CompletedAt:   completedAt,
			LiveCheckedAt: parseOptionalTime(liveCheckedAtRaw),
			UpdatedAt:     parseSQLiteTime(updatedAtRaw),
			CheckedByID:   &checkedByID,
			CheckedByName: checkedByName,
			CanToggle:     canToggle,
			CanRefresh:    source == domain.CompletionSourceLive,
		}
	}
	return result, rows.Err()
}

func completionStatusFromCompleted(source string, completed bool) string {
	if source == domain.CompletionSourceLive {
		if completed {
			return domain.CompletionStatusLiveCompleted
		}
		return domain.CompletionStatusLiveIncomplete
	}
	if completed {
		return domain.CompletionStatusManualCompleted
	}
	return domain.CompletionStatusManualIncomplete
}

func parseSQLiteTime(value string) *time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339Nano, time.RFC3339} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return &parsed
		}
	}
	return nil
}

func parseOptionalTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	return parseSQLiteTime(value)
}

func formatOptionalTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
