package tasksync

import (
	"context"
	"strings"
	"time"

	"bws-checkin/backend/internal/store"
)

type Task struct {
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

type Source interface {
	FetchTasks(ctx context.Context) ([]Task, error)
}

type Config struct {
	Now      func() time.Time
	FreshTTL time.Duration
	Async    func(func())
}

type Syncer struct {
	store    *store.Store
	source   Source
	now      func() time.Time
	freshTTL time.Duration
	async    func(func())
}

func New(st *store.Store, source Source, config Config) *Syncer {
	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	freshTTL := config.FreshTTL
	if freshTTL <= 0 {
		freshTTL = 5 * time.Minute
	}
	async := config.Async
	if async == nil {
		async = func(fn func()) { go fn() }
	}
	return &Syncer{store: st, source: source, now: now, freshTTL: freshTTL, async: async}
}

func (s *Syncer) Sync(ctx context.Context) error {
	tasks, err := s.source.FetchTasks(ctx)
	if err != nil {
		_ = s.store.RecordTaskSyncError(ctx, errorCode(err), s.now())
		return err
	}
	inputs := make([]store.SyncedTaskInput, 0, len(tasks))
	for index, task := range tasks {
		id := taskID(task)
		if id == "" || strings.TrimSpace(task.Name) == "" {
			continue
		}
		sortOrder := task.SortOrder
		if sortOrder == 0 {
			sortOrder = (index + 1) * 10
		}
		title := task.Title
		if title == "" {
			title = task.Name
		}
		inputs = append(inputs, store.SyncedTaskInput{
			ID:          id,
			ExternalID:  task.ExternalID,
			GroupName:   task.GroupName,
			Name:        task.Name,
			Title:       title,
			RewardCoins: task.RewardCoins,
			Description: task.Description,
			ImageURL:    task.ImageURL,
			VenueID:     task.VenueID,
			VenueName:   task.VenueName,
			EventDay:    task.EventDay,
			SortOrder:   sortOrder,
		})
	}
	if len(inputs) == 0 {
		_ = s.store.RecordTaskSyncError(ctx, "empty_task_list", s.now())
		return nil
	}
	if err := s.store.ReplaceBilibiliTasks(ctx, inputs); err != nil {
		_ = s.store.RecordTaskSyncError(ctx, "store_write_failed", s.now())
		return err
	}
	return s.store.RecordTaskSyncSuccess(ctx, s.now())
}

func (s *Syncer) EnsureFresh(ctx context.Context) bool {
	state, err := s.store.TaskSyncState(ctx)
	if err == nil && state.LastSuccessAt != nil && s.now().Sub(*state.LastSuccessAt) < s.freshTTL {
		return true
	}
	s.async(func() {
		_ = s.Sync(context.Background())
	})
	return false
}

func taskID(task Task) string {
	externalID := strings.TrimSpace(task.ExternalID)
	eventDay := strings.TrimSpace(task.EventDay)
	venueID := strings.TrimSpace(task.VenueID)
	if externalID == "" || eventDay == "" || venueID == "" {
		return ""
	}
	return "bilibili:" + externalID + ":" + eventDay + ":" + venueID
}

func errorCode(err error) string {
	text := strings.ToLower(err.Error())
	replacer := strings.NewReplacer(" ", "_", "-", "_", ":", "", ".", "", "/", "_")
	code := replacer.Replace(text)
	if code == "" {
		return "sync_failed"
	}
	if len(code) > 64 {
		return code[:64]
	}
	return code
}
