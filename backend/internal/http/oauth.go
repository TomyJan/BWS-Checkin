package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"bws-checkin/backend/internal/domain"
	"github.com/go-chi/chi/v5"
)

const oauthStateCookiePrefix = "bws_oauth_state_"

type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type oauthProfile struct {
	Subject     string
	DisplayName string
	AvatarURL   string
}

type qqOpenIDResponse struct {
	ClientID string `json:"client_id"`
	OpenID   string `json:"openid"`
}

type qqUserInfoResponse struct {
	ReturnCode   int    `json:"ret"`
	Nickname     string `json:"nickname"`
	FigureURLQQ2 string `json:"figureurl_qq_2"`
	FigureURLQQ1 string `json:"figureurl_qq_1"`
}

func (h Handler) oauthLogin(w http.ResponseWriter, r *http.Request) {
	provider, ok := h.oauthProvider(chi.URLParam(r, "provider"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	state, err := randomState()
	if err != nil {
		http.Error(w, "failed to create OAuth state", http.StatusInternalServerError)
		return
	}
	setOAuthState(w, provider.ID, state)

	authURL, err := h.oauthAuthorizationURL(r, provider, state)
	if err != nil {
		http.Error(w, "OAuth provider is not configured", http.StatusServiceUnavailable)
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h Handler) oauthCallback(w http.ResponseWriter, r *http.Request) {
	provider, ok := h.oauthProvider(chi.URLParam(r, "provider"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := validateOAuthState(r, provider.ID); err != nil {
		http.Error(w, "invalid OAuth state", http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing OAuth code", http.StatusBadRequest)
		return
	}

	profile, err := h.oauthProfile(r, provider, code)
	if err != nil {
		http.Error(w, "OAuth profile failed", http.StatusBadGateway)
		return
	}
	user, err := h.loginOrBindOAuthProfile(r, provider, profile)
	if err != nil {
		http.Error(w, "OAuth account binding failed", http.StatusBadRequest)
		return
	}
	h.setSession(w, user.ID)
	clearOAuthState(w, provider.ID)
	http.Redirect(w, r, h.postLoginRedirect(), http.StatusFound)
}

func (h Handler) oauthProvider(id string) (OAuthProviderConfig, bool) {
	for _, provider := range h.deps.OAuthProviders {
		if provider.ID == id && provider.ID != "" {
			return provider, true
		}
	}
	return OAuthProviderConfig{}, false
}

func (h Handler) oauthAuthorizationURL(r *http.Request, provider OAuthProviderConfig, state string) (string, error) {
	authURL := provider.AuthURL
	if provider.providerType() == "oidc" && authURL == "" {
		oidcHandler := h.withOIDCProvider(provider)
		discovery, err := oidcHandler.oidcDiscovery(r)
		if err != nil {
			return "", err
		}
		authURL = discovery.AuthorizationEndpoint
	}
	if authURL == "" || provider.ClientID == "" || provider.RedirectURL == "" {
		return "", errors.New("oauth provider is incomplete")
	}
	values := url.Values{}
	values.Set("client_id", provider.ClientID)
	values.Set("redirect_uri", provider.RedirectURL)
	values.Set("response_type", "code")
	values.Set("scope", provider.oauthScope())
	values.Set("state", state)

	separator := "?"
	if strings.Contains(authURL, "?") {
		separator = "&"
	}
	return authURL + separator + values.Encode(), nil
}

func (h Handler) oauthProfile(r *http.Request, provider OAuthProviderConfig, code string) (oauthProfile, error) {
	switch provider.providerType() {
	case "qq":
		return h.qqOAuthProfile(r, provider, code)
	case "oidc":
		return h.oidcOAuthProfile(r, provider, code)
	default:
		return oauthProfile{}, errors.New("unsupported oauth provider type")
	}
}

func (h Handler) oidcOAuthProfile(r *http.Request, provider OAuthProviderConfig, code string) (oauthProfile, error) {
	oidcHandler := h.withOIDCProvider(provider)
	discovery, err := oidcHandler.oidcDiscovery(r)
	if err != nil {
		return oauthProfile{}, err
	}
	token, err := oidcHandler.exchangeOIDCCode(r, discovery, code)
	if err != nil {
		return oauthProfile{}, err
	}
	idTokenSubject, err := oidcHandler.validateOIDCIDToken(r, discovery, token.IDToken)
	if err != nil {
		return oauthProfile{}, err
	}
	userinfo, err := oidcHandler.loadOIDCUserinfo(r, discovery, token.AccessToken)
	if err != nil {
		return oauthProfile{}, err
	}
	displayName := userinfo.displayName()
	if displayName == "" || userinfo.Subject == "" || userinfo.Subject != idTokenSubject {
		return oauthProfile{}, errors.New("oidc userinfo is incomplete")
	}
	return oauthProfile{Subject: userinfo.Subject, DisplayName: displayName, AvatarURL: userinfo.Picture}, nil
}

func (h Handler) qqOAuthProfile(r *http.Request, provider OAuthProviderConfig, code string) (oauthProfile, error) {
	token, err := exchangeOAuthCode(r, provider, code)
	if err != nil {
		return oauthProfile{}, err
	}
	openID, err := loadQQOpenID(r, provider, token.AccessToken)
	if err != nil {
		return oauthProfile{}, err
	}
	userinfo, err := loadQQUserInfo(r, provider, token.AccessToken, openID.OpenID)
	if err != nil {
		return oauthProfile{}, err
	}
	if openID.OpenID == "" || userinfo.Nickname == "" {
		return oauthProfile{}, errors.New("qq userinfo is incomplete")
	}
	avatarURL := userinfo.FigureURLQQ2
	if avatarURL == "" {
		avatarURL = userinfo.FigureURLQQ1
	}
	return oauthProfile{Subject: openID.OpenID, DisplayName: userinfo.Nickname, AvatarURL: avatarURL}, nil
}

func exchangeOAuthCode(r *http.Request, provider OAuthProviderConfig, code string) (oauthTokenResponse, error) {
	if provider.TokenURL == "" || provider.ClientID == "" || provider.RedirectURL == "" {
		return oauthTokenResponse{}, errors.New("oauth token config is incomplete")
	}
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("code", code)
	values.Set("redirect_uri", provider.RedirectURL)
	values.Set("client_id", provider.ClientID)
	if provider.ClientSecret != "" {
		values.Set("client_secret", provider.ClientSecret)
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, provider.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return oauthTokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return oauthTokenResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return oauthTokenResponse{}, errors.New("token endpoint failed")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return oauthTokenResponse{}, err
	}
	var token oauthTokenResponse
	if strings.Contains(response.Header.Get("Content-Type"), "application/json") {
		if err := json.Unmarshal(body, &token); err != nil {
			return oauthTokenResponse{}, err
		}
	} else {
		values, err := url.ParseQuery(string(body))
		if err != nil {
			return oauthTokenResponse{}, err
		}
		token.AccessToken = values.Get("access_token")
		token.TokenType = values.Get("token_type")
	}
	if token.AccessToken == "" {
		return oauthTokenResponse{}, errors.New("missing access token")
	}
	return token, nil
}

func loadQQOpenID(r *http.Request, provider OAuthProviderConfig, accessToken string) (qqOpenIDResponse, error) {
	endpoint, err := qqOpenIDURL(provider.UserInfoURL)
	if err != nil {
		return qqOpenIDResponse{}, err
	}
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return qqOpenIDResponse{}, err
	}
	values := reqURL.Query()
	values.Set("access_token", accessToken)
	reqURL.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return qqOpenIDResponse{}, err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return qqOpenIDResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return qqOpenIDResponse{}, errors.New("qq openid endpoint failed")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return qqOpenIDResponse{}, err
	}
	var openID qqOpenIDResponse
	if err := json.Unmarshal(jsonpBody(body), &openID); err != nil {
		return qqOpenIDResponse{}, err
	}
	return openID, nil
}

func loadQQUserInfo(r *http.Request, provider OAuthProviderConfig, accessToken string, openID string) (qqUserInfoResponse, error) {
	if provider.UserInfoURL == "" || provider.ClientID == "" || openID == "" {
		return qqUserInfoResponse{}, errors.New("qq userinfo config is incomplete")
	}
	reqURL, err := url.Parse(provider.UserInfoURL)
	if err != nil {
		return qqUserInfoResponse{}, err
	}
	values := reqURL.Query()
	values.Set("access_token", accessToken)
	values.Set("oauth_consumer_key", provider.ClientID)
	values.Set("openid", openID)
	reqURL.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return qqUserInfoResponse{}, err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return qqUserInfoResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return qqUserInfoResponse{}, errors.New("qq userinfo endpoint failed")
	}
	var userinfo qqUserInfoResponse
	if err := json.NewDecoder(response.Body).Decode(&userinfo); err != nil {
		return qqUserInfoResponse{}, err
	}
	if userinfo.ReturnCode != 0 {
		return qqUserInfoResponse{}, errors.New("qq userinfo returned error")
	}
	return userinfo, nil
}

func (h Handler) loginOrBindOAuthProfile(r *http.Request, provider OAuthProviderConfig, profile oauthProfile) (domain.User, error) {
	if profile.Subject == "" || profile.DisplayName == "" {
		return domain.User{}, errors.New("oauth profile is incomplete")
	}
	currentUserID, hasCurrentSession := h.sessionUserID(r)
	linked, err := h.deps.Store.OAuthAccountByProviderSubject(r.Context(), provider.ID, profile.Subject)
	if err == nil {
		if hasCurrentSession && linked.UserID != currentUserID {
			return domain.User{}, errors.New("oauth account already linked")
		}
		user, err := h.deps.Store.UserByID(r.Context(), linked.UserID)
		if err != nil {
			return domain.User{}, err
		}
		return user, h.deps.Store.LinkOAuthAccount(r.Context(), domain.OAuthAccount{
			ProviderID:  provider.ID,
			UserID:      user.ID,
			Subject:     profile.Subject,
			DisplayName: profile.DisplayName,
			AvatarURL:   profile.AvatarURL,
		})
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, err
	}

	var user domain.User
	if hasCurrentSession {
		loaded, err := h.deps.Store.UserByID(r.Context(), currentUserID)
		if err != nil {
			return domain.User{}, err
		}
		user = loaded
	} else {
		created, err := h.deps.Store.UpsertUser(r.Context(), "oauth:"+provider.ID+":"+profile.Subject, profile.DisplayName)
		if err != nil {
			return domain.User{}, err
		}
		user = created
	}
	if err := h.deps.Store.LinkOAuthAccount(r.Context(), domain.OAuthAccount{
		ProviderID:  provider.ID,
		UserID:      user.ID,
		Subject:     profile.Subject,
		DisplayName: profile.DisplayName,
		AvatarURL:   profile.AvatarURL,
	}); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (h Handler) withOIDCProvider(provider OAuthProviderConfig) Handler {
	oidcHandler := h
	oidcHandler.deps.OIDC = OIDCConfig{
		IssuerURL:         provider.IssuerURL,
		ClientID:          provider.ClientID,
		ClientSecret:      provider.ClientSecret,
		RedirectURL:       provider.RedirectURL,
		PostLoginRedirect: h.postLoginRedirect(),
	}
	return oidcHandler
}

func setOAuthState(w http.ResponseWriter, providerID string, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookiePrefix + providerID,
		Value:    state,
		Path:     "/api/v1/auth/oauth/" + providerID,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func validateOAuthState(r *http.Request, providerID string) error {
	cookie, err := r.Cookie(oauthStateCookiePrefix + providerID)
	if err != nil {
		return err
	}
	if cookie.Value == "" || cookie.Value != r.URL.Query().Get("state") {
		return errors.New("oauth state mismatch")
	}
	return nil
}

func clearOAuthState(w http.ResponseWriter, providerID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookiePrefix + providerID,
		Value:    "",
		Path:     "/api/v1/auth/oauth/" + providerID,
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (c OAuthProviderConfig) providerType() string {
	if c.Type == "" {
		return "oidc"
	}
	return strings.ToLower(c.Type)
}

func (c OAuthProviderConfig) displayName() string {
	if c.Name != "" {
		return c.Name
	}
	return c.ID
}

func (c OAuthProviderConfig) oauthScope() string {
	if c.providerType() == "qq" {
		return "get_user_info"
	}
	return "openid profile email"
}

func qqOpenIDURL(userInfoURL string) (string, error) {
	parsed, err := url.Parse(userInfoURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("qq userinfo url is invalid")
	}
	if parsed.Host == "graph.qq.com" {
		parsed.Path = "/oauth2.0/me"
	} else {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/userinfo") + "/me"
	}
	parsed.RawQuery = ""
	return parsed.String(), nil
}

func jsonpBody(body []byte) []byte {
	value := strings.TrimSpace(string(body))
	start := strings.Index(value, "{")
	end := strings.LastIndex(value, "}")
	if start >= 0 && end >= start {
		value = value[start : end+1]
	}
	return []byte(value)
}
