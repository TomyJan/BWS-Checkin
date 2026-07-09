package domain

import "time"

type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl"`
	QRImageURL  string `json:"qrImageUrl"`
	QRSource    string `json:"qrSource"`
}

const (
	QRSourceUploaded          = "uploaded"
	QRSourceBilibiliGenerated = "bilibili_generated"
)

type BilibiliAccount struct {
	UserID                 string     `json:"userId"`
	MID                    string     `json:"mid"`
	Uname                  string     `json:"uname"`
	FaceURL                string     `json:"faceUrl"`
	CookieCiphertext       string     `json:"-"`
	CookieExpiresAt        *time.Time `json:"cookieExpiresAt"`
	RefreshTokenCiphertext string     `json:"-"`
	LastValidatedAt        *time.Time `json:"lastValidatedAt"`
	CreatedAt              *time.Time `json:"createdAt"`
	UpdatedAt              *time.Time `json:"updatedAt"`
}

type OAuthProvider struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type OAuthAccount struct {
	ProviderID   string     `json:"providerId"`
	ProviderName string     `json:"providerName"`
	UserID       string     `json:"userId"`
	Subject      string     `json:"subject"`
	DisplayName  string     `json:"displayName"`
	AvatarURL    string     `json:"avatarUrl"`
	CreatedAt    *time.Time `json:"createdAt"`
	UpdatedAt    *time.Time `json:"updatedAt"`
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
	GroupName      string             `json:"groupName"`
	Name           string             `json:"name"`
	Title          string             `json:"title"`
	RewardCoins    int                `json:"rewardCoins"`
	Description    string             `json:"description"`
	ExternalID     string             `json:"externalId"`
	ImageURL       string             `json:"imageUrl"`
	VenueID        string             `json:"venueId"`
	VenueName      string             `json:"venueName"`
	EventDay       string             `json:"eventDay"`
	SyncSource     string             `json:"syncSource"`
	SortOrder      int                `json:"sortOrder"`
	CompletedCount int                `json:"completedCount"`
	TotalCount     int                `json:"totalCount"`
	Members        []MemberCompletion `json:"members"`
}

const (
	CompletionStatusManualIncomplete = "manual_incomplete"
	CompletionStatusManualCompleted  = "manual_completed"
	CompletionStatusLiveIncomplete   = "live_incomplete"
	CompletionStatusLiveCompleted    = "live_completed"

	CompletionSourceManual = "manual"
	CompletionSourceLive   = "live"
)

type MemberCompletion struct {
	Member        Member     `json:"member"`
	Completed     bool       `json:"completed"`
	Status        string     `json:"status"`
	Source        string     `json:"source"`
	LiveStale     bool       `json:"liveStale"`
	CompletedAt   *time.Time `json:"completedAt"`
	LiveCheckedAt *time.Time `json:"liveCheckedAt"`
	UpdatedAt     *time.Time `json:"updatedAt"`
	CheckedByID   *string    `json:"checkedById"`
	CheckedByName string     `json:"checkedByName"`
	CanToggle     bool       `json:"canToggle"`
	CanRefresh    bool       `json:"canRefresh"`
}
