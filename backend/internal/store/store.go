package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"os"
	"path/filepath"
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

type CreateGroupInput struct {
	ID          string
	Name        string
	Day         string
	Description string
	OwnerUserID int64
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
	_, err = s.db.Exec(string(body))
	return err
}

func (s *Store) UpsertUser(ctx context.Context, subject, displayName string) (domain.User, error) {
	if subject == "" || displayName == "" {
		return domain.User{}, errors.New("subject and display name are required")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (oidc_subject, display_name)
		VALUES (?, ?)
		ON CONFLICT(oidc_subject) DO UPDATE SET display_name = excluded.display_name, updated_at = CURRENT_TIMESTAMP
	`, subject, displayName)
	if err != nil {
		return domain.User{}, err
	}
	return s.UserBySubject(ctx, subject)
}

func (s *Store) UserBySubject(ctx context.Context, subject string) (domain.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `
		SELECT id, display_name, avatar_url, qr_image_path
		FROM users
		WHERE oidc_subject = ?
	`, subject))
}

func (s *Store) UserByID(ctx context.Context, id int64) (domain.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `
		SELECT id, display_name, avatar_url, qr_image_path
		FROM users
		WHERE id = ?
	`, id))
}

func (s *Store) UpdateUserQR(ctx context.Context, userID int64, path string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET qr_image_path = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, path, userID)
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

func (s *Store) JoinGroup(ctx context.Context, groupID string, userID int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO group_members (group_id, user_id, role)
		VALUES (?, ?, 'member')
	`, groupID, userID)
	return err
}

func (s *Store) UserGroups(ctx context.Context, userID int64) ([]domain.Group, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.id, g.name, g.day, g.description, gm.role,
			(SELECT COUNT(*) FROM group_members WHERE group_id = g.id) AS member_count,
			(SELECT COUNT(*) FROM tasks WHERE enabled = 1) AS task_count
		FROM groups g
		JOIN group_members gm ON gm.group_id = g.id
		WHERE gm.user_id = ?
		ORDER BY g.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []domain.Group
	for rows.Next() {
		var group domain.Group
		if err := rows.Scan(&group.ID, &group.Name, &group.Day, &group.Description, &group.Role, &group.MemberCount, &group.TaskCount); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (s *Store) GroupTasks(ctx context.Context, groupID string) ([]domain.TaskStatus, error) {
	members, err := s.groupMembers(ctx, groupID)
	if err != nil {
		return nil, err
	}
	tasks, err := s.enabledTasks(ctx)
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
			item := domain.MemberCompletion{Member: member}
			if completion, ok := completions[completionKey{TaskID: tasks[taskIndex].ID, UserID: member.ID}]; ok {
				item = completion
				item.Member = member
				tasks[taskIndex].CompletedCount++
			}
			tasks[taskIndex].Members = append(tasks[taskIndex].Members, item)
		}
	}
	return tasks, nil
}

func (s *Store) MarkComplete(ctx context.Context, groupID string, taskID string, targetUserID int64, checkedByUserID int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO task_completions (group_id, task_id, target_user_id, checked_by_user_id)
		VALUES (?, ?, ?, ?)
	`, groupID, taskID, targetUserID, checkedByUserID)
	return err
}

func scanUser(row *sql.Row) (domain.User, error) {
	var user domain.User
	if err := row.Scan(&user.ID, &user.DisplayName, &user.AvatarURL, &user.QRImageURL); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *Store) groupMembers(ctx context.Context, groupID string) ([]domain.Member, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.display_name, u.qr_image_path
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
		if err := rows.Scan(&member.ID, &member.DisplayName, &member.QRImageURL); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (s *Store) enabledTasks(ctx context.Context) ([]domain.TaskStatus, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, sort_order
		FROM tasks
		WHERE enabled = 1
		ORDER BY sort_order ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []domain.TaskStatus
	for rows.Next() {
		var task domain.TaskStatus
		if err := rows.Scan(&task.ID, &task.Name, &task.SortOrder); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

type completionKey struct {
	TaskID string
	UserID int64
}

func (s *Store) completions(ctx context.Context, groupID string) (map[completionKey]domain.MemberCompletion, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tc.task_id, tc.target_user_id, tc.checked_by_user_id, u.display_name, tc.completed_at
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
		var targetUserID int64
		var checkedByID int64
		var checkedByName string
		var completedAtRaw string
		if err := rows.Scan(&taskID, &targetUserID, &checkedByID, &checkedByName, &completedAtRaw); err != nil {
			return nil, err
		}
		completedAt := parseSQLiteTime(completedAtRaw)
		result[completionKey{TaskID: taskID, UserID: targetUserID}] = domain.MemberCompletion{
			Completed:     true,
			CompletedAt:   completedAt,
			CheckedByID:   &checkedByID,
			CheckedByName: checkedByName,
		}
	}
	return result, rows.Err()
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
