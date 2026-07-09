package domain

import "time"

type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl"`
	QRImageURL  string `json:"qrImageUrl"`
}

type Group struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Day         string     `json:"day"`
	Description string     `json:"description"`
	Role        string     `json:"role"`
	MemberCount int        `json:"memberCount"`
	TaskCount   int        `json:"taskCount"`
	JoinLocked  bool       `json:"joinLocked"`
	ArchivedAt  *time.Time `json:"archivedAt"`
}

type Member struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	QRImageURL  string `json:"qrImageUrl"`
}

type TaskStatus struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	SortOrder      int                `json:"sortOrder"`
	CompletedCount int                `json:"completedCount"`
	TotalCount     int                `json:"totalCount"`
	Members        []MemberCompletion `json:"members"`
}

type MemberCompletion struct {
	Member        Member     `json:"member"`
	Completed     bool       `json:"completed"`
	CompletedAt   *time.Time `json:"completedAt"`
	UpdatedAt     *time.Time `json:"updatedAt"`
	CheckedByID   *string    `json:"checkedById"`
	CheckedByName string     `json:"checkedByName"`
}
