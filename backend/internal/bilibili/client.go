package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPassportBaseURL = "https://passport.bilibili.com"
	defaultAPIBaseURL      = "https://api.bilibili.com"

	LoginStatusPendingScan    = "pending_scan"
	LoginStatusPendingConfirm = "pending_confirm"
	LoginStatusExpired        = "expired"
	LoginStatusConfirmed      = "confirmed"
	LoginStatusFailed         = "failed"
)

type Client struct {
	passportBaseURL string
	apiBaseURL      string
	httpClient      *http.Client
}

type ClientOptions struct {
	PassportBaseURL string
	APIBaseURL      string
	HTTPClient      *http.Client
}

type LoginQRCode struct {
	URL       string
	QRCodeKey string
	ExpiresAt time.Time
}

type LoginPollResult struct {
	Status       string
	Message      string
	RefreshToken string
	Cookies      []*http.Cookie
}

type NavUser struct {
	MID     string
	Uname   string
	FaceURL string
}

type OfflinePointsRequest struct {
	BID     int
	Year    int
	VenueID int
	Day     string
}

type OfflinePoint struct {
	ID          string
	Name        string
	ImageURL    string
	RewardCoins int
	Completed   bool
	Description string
	EventDay    string
}

func NewClient(options ClientOptions) *Client {
	passportBaseURL := strings.TrimRight(options.PassportBaseURL, "/")
	if passportBaseURL == "" {
		passportBaseURL = defaultPassportBaseURL
	}
	apiBaseURL := strings.TrimRight(options.APIBaseURL, "/")
	if apiBaseURL == "" {
		apiBaseURL = defaultAPIBaseURL
	}
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		passportBaseURL: passportBaseURL,
		apiBaseURL:      apiBaseURL,
		httpClient:      httpClient,
	}
}

func (c *Client) CreateLoginQRCode(ctx context.Context) (LoginQRCode, error) {
	var payload struct {
		Code int `json:"code"`
		Data struct {
			URL       string `json:"url"`
			QRCodeKey string `json:"qrcode_key"`
		} `json:"data"`
	}
	if err := c.getJSON(ctx, c.passportBaseURL+"/x/passport-login/web/qrcode/generate", nil, &payload); err != nil {
		return LoginQRCode{}, err
	}
	if payload.Code != 0 {
		return LoginQRCode{}, fmt.Errorf("bilibili qrcode create failed: code %d", payload.Code)
	}
	if payload.Data.URL == "" || payload.Data.QRCodeKey == "" {
		return LoginQRCode{}, fmt.Errorf("bilibili qrcode create failed: missing data")
	}
	return LoginQRCode{
		URL:       payload.Data.URL,
		QRCodeKey: payload.Data.QRCodeKey,
		ExpiresAt: time.Now().UTC().Add(3 * time.Minute),
	}, nil
}

func (c *Client) PollLoginQRCode(ctx context.Context, qrcodeKey string) (LoginPollResult, error) {
	endpoint, err := url.Parse(c.passportBaseURL + "/x/passport-login/web/qrcode/poll")
	if err != nil {
		return LoginPollResult{}, err
	}
	query := endpoint.Query()
	query.Set("qrcode_key", qrcodeKey)
	endpoint.RawQuery = query.Encode()

	var payload struct {
		Code int `json:"code"`
		Data struct {
			Code         int    `json:"code"`
			Message      string `json:"message"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	res, err := c.getJSONResponse(ctx, endpoint.String(), nil, &payload)
	if err != nil {
		return LoginPollResult{}, err
	}
	if payload.Code != 0 {
		return LoginPollResult{}, fmt.Errorf("bilibili qrcode poll failed: code %d", payload.Code)
	}
	return LoginPollResult{
		Status:       mapLoginStatus(payload.Data.Code),
		Message:      payload.Data.Message,
		RefreshToken: payload.Data.RefreshToken,
		Cookies:      res.Cookies(),
	}, nil
}

func (c *Client) Nav(ctx context.Context, cookies []*http.Cookie) (NavUser, error) {
	var payload struct {
		Code int `json:"code"`
		Data struct {
			IsLogin bool   `json:"isLogin"`
			MID     int64  `json:"mid"`
			Uname   string `json:"uname"`
			Face    string `json:"face"`
		} `json:"data"`
	}
	if err := c.getJSON(ctx, c.apiBaseURL+"/x/web-interface/nav", cookies, &payload); err != nil {
		return NavUser{}, err
	}
	if payload.Code != 0 || !payload.Data.IsLogin || payload.Data.MID == 0 {
		return NavUser{}, fmt.Errorf("bilibili nav unauthorized")
	}
	return NavUser{
		MID:     strconv.FormatInt(payload.Data.MID, 10),
		Uname:   payload.Data.Uname,
		FaceURL: payload.Data.Face,
	}, nil
}

func (c *Client) OfflinePoints(ctx context.Context, request OfflinePointsRequest, cookies []*http.Cookie) ([]OfflinePoint, error) {
	endpoint, err := url.Parse(c.apiBaseURL + "/x/activity/bws/offline/points")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("bid", strconv.Itoa(request.BID))
	query.Set("fid", strconv.Itoa(request.VenueID))
	query.Set("day", request.Day)
	query.Set("year", strconv.Itoa(request.Year))
	endpoint.RawQuery = query.Encode()

	var payload struct {
		Code int `json:"code"`
		Data struct {
			PointsList map[string]struct {
				Points []struct {
					ID          any    `json:"id"`
					Name        string `json:"name"`
					Image       string `json:"image"`
					RewardCoins int    `json:"unlocked"`
					IsPoint     any    `json:"is_point"`
					Description string `json:"dic"`
				} `json:"points"`
			} `json:"points_list"`
		} `json:"data"`
	}
	if err := c.getJSON(ctx, endpoint.String(), cookies, &payload); err != nil {
		return nil, err
	}
	if payload.Code != 0 {
		return nil, fmt.Errorf("bilibili offline points failed: code %d", payload.Code)
	}
	var points []OfflinePoint
	for day, dayPoints := range payload.Data.PointsList {
		for _, point := range dayPoints.Points {
			id := stringify(point.ID)
			if id == "" {
				continue
			}
			points = append(points, OfflinePoint{
				ID:          id,
				Name:        point.Name,
				ImageURL:    point.Image,
				RewardCoins: point.RewardCoins,
				Completed:   truthy(point.IsPoint),
				Description: point.Description,
				EventDay:    day,
			})
		}
	}
	return points, nil
}

func (c *Client) getJSON(ctx context.Context, endpoint string, cookies []*http.Cookie, target any) error {
	_, err := c.getJSONResponse(ctx, endpoint, cookies, target)
	return err
}

func (c *Client) getJSONResponse(ctx context.Context, endpoint string, cookies []*http.Cookie, target any) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("bilibili request failed: status %d", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		return nil, err
	}
	return res, nil
}

func mapLoginStatus(code int) string {
	switch code {
	case 0:
		return LoginStatusConfirmed
	case 86101:
		return LoginStatusPendingScan
	case 86090:
		return LoginStatusPendingConfirm
	case 86038:
		return LoginStatusExpired
	default:
		return LoginStatusFailed
	}
}

func stringify(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return ""
	}
}

func truthy(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case int:
		return v != 0
	case string:
		return v == "1" || strings.EqualFold(v, "true")
	default:
		return false
	}
}
