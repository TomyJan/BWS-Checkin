package tasksync

import (
	"context"
	"strconv"

	"bws-checkin/backend/internal/bilibili"
	"bws-checkin/backend/internal/domain"
	"bws-checkin/backend/internal/store"
)

const (
	defaultBID  = 202601
	defaultYear = 202601
)

type Venue struct {
	ID   int
	Name string
}

type BilibiliSourceConfig struct {
	Store        *store.Store
	Client       *bilibili.Client
	CookieSecret string
	BID          int
	Year         int
	Days         []string
	Venues       []Venue
}

type BilibiliSource struct {
	store        *store.Store
	client       *bilibili.Client
	cookieSecret string
	bid          int
	year         int
	days         []string
	venues       []Venue
}

func NewBilibiliSource(config BilibiliSourceConfig) *BilibiliSource {
	bid := config.BID
	if bid == 0 {
		bid = defaultBID
	}
	year := config.Year
	if year == 0 {
		year = defaultYear
	}
	days := config.Days
	if len(days) == 0 {
		days = []string{"20260710", "20260711", "20260712"}
	}
	venues := config.Venues
	if len(venues) == 0 {
		venues = []Venue{
			{ID: 1, Name: "8.1馆"},
			{ID: 2, Name: "1.1馆"},
			{ID: 3, Name: "2.1馆"},
			{ID: 4, Name: "3馆"},
			{ID: 5, Name: "4.1馆"},
			{ID: 7, Name: "5.1馆"},
			{ID: 6, Name: "6.1馆"},
		}
	}
	return &BilibiliSource{
		store:        config.Store,
		client:       config.Client,
		cookieSecret: config.CookieSecret,
		bid:          bid,
		year:         year,
		days:         days,
		venues:       venues,
	}
}

func (s *BilibiliSource) FetchTasks(ctx context.Context) ([]Task, error) {
	account, err := s.store.AnyBilibiliAccount(ctx)
	if err != nil {
		return nil, err
	}
	return s.FetchTasksForAccount(ctx, account)
}

func (s *BilibiliSource) FetchTasksForAccount(ctx context.Context, account domain.BilibiliAccount) ([]Task, error) {
	cookies, err := bilibili.DecryptCookieJar(s.cookieSecret, account.CookieCiphertext)
	if err != nil {
		return nil, err
	}
	var tasks []Task
	sortOrder := 10
	for _, venue := range s.venues {
		for _, day := range s.days {
			points, err := s.client.OfflinePoints(ctx, bilibili.OfflinePointsRequest{
				BID:     s.bid,
				Year:    s.year,
				VenueID: venue.ID,
				Day:     day,
			}, cookies)
			if err != nil {
				return nil, err
			}
			for _, point := range points {
				tasks = append(tasks, Task{
					ExternalID:  point.ID,
					GroupName:   venue.Name,
					Name:        point.Name,
					Title:       point.Name,
					RewardCoins: point.RewardCoins,
					Description: point.Description,
					ImageURL:    point.ImageURL,
					VenueID:     strconv.Itoa(venue.ID),
					VenueName:   venue.Name,
					EventDay:    point.EventDay,
					SortOrder:   sortOrder,
				})
				sortOrder += 10
			}
		}
	}
	return tasks, nil
}
