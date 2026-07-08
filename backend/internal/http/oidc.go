package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

const oidcStateCookieName = "bws_oidc_state"

type OIDCConfig struct {
	IssuerURL         string
	ClientID          string
	ClientSecret      string
	RedirectURL       string
	PostLoginRedirect string
}

type oidcDiscovery struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
}

type oidcTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type oidcUserinfo struct {
	Subject           string `json:"sub"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	Picture           string `json:"picture"`
}

func (h Handler) oidcLogin(w http.ResponseWriter, r *http.Request) {
	discovery, err := h.oidcDiscovery(r)
	if err != nil {
		http.Error(w, "OIDC is not configured", http.StatusServiceUnavailable)
		return
	}
	state, err := randomState()
	if err != nil {
		http.Error(w, "failed to create OIDC state", http.StatusInternalServerError)
		return
	}
	setOIDCState(w, state)

	values := url.Values{}
	values.Set("client_id", h.deps.OIDC.ClientID)
	values.Set("redirect_uri", h.deps.OIDC.RedirectURL)
	values.Set("response_type", "code")
	values.Set("scope", "openid profile email")
	values.Set("state", state)

	redirectURL := discovery.AuthorizationEndpoint
	separator := "?"
	if strings.Contains(redirectURL, "?") {
		separator = "&"
	}
	http.Redirect(w, r, redirectURL+separator+values.Encode(), http.StatusFound)
}

func (h Handler) oidcCallback(w http.ResponseWriter, r *http.Request) {
	if err := h.validateOIDCState(r); err != nil {
		http.Error(w, "invalid OIDC state", http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing OIDC code", http.StatusBadRequest)
		return
	}
	discovery, err := h.oidcDiscovery(r)
	if err != nil {
		http.Error(w, "OIDC is not configured", http.StatusServiceUnavailable)
		return
	}
	token, err := h.exchangeOIDCCode(r, discovery, code)
	if err != nil {
		http.Error(w, "OIDC token exchange failed", http.StatusBadGateway)
		return
	}
	userinfo, err := h.loadOIDCUserinfo(r, discovery, token.AccessToken)
	if err != nil {
		http.Error(w, "OIDC userinfo failed", http.StatusBadGateway)
		return
	}
	displayName := userinfo.displayName()
	if displayName == "" || userinfo.Subject == "" {
		http.Error(w, "OIDC userinfo is incomplete", http.StatusBadGateway)
		return
	}
	user, err := h.deps.Store.UpsertUser(r.Context(), "oidc:"+userinfo.Subject, displayName)
	if err != nil {
		http.Error(w, "failed to upsert OIDC user", http.StatusInternalServerError)
		return
	}
	setSession(w, user.ID)
	clearOIDCState(w)
	http.Redirect(w, r, h.postLoginRedirect(), http.StatusFound)
}

func (h Handler) oidcDiscovery(r *http.Request) (oidcDiscovery, error) {
	if h.deps.OIDC.IssuerURL == "" || h.deps.OIDC.ClientID == "" || h.deps.OIDC.RedirectURL == "" {
		return oidcDiscovery{}, errors.New("oidc config is incomplete")
	}
	endpoint := strings.TrimRight(h.deps.OIDC.IssuerURL, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		return oidcDiscovery{}, err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return oidcDiscovery{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return oidcDiscovery{}, errors.New("oidc discovery failed")
	}
	var discovery oidcDiscovery
	if err := json.NewDecoder(response.Body).Decode(&discovery); err != nil {
		return oidcDiscovery{}, err
	}
	if discovery.AuthorizationEndpoint == "" || discovery.TokenEndpoint == "" || discovery.UserinfoEndpoint == "" {
		return oidcDiscovery{}, errors.New("oidc discovery is incomplete")
	}
	return discovery, nil
}

func (h Handler) exchangeOIDCCode(r *http.Request, discovery oidcDiscovery, code string) (oidcTokenResponse, error) {
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("code", code)
	values.Set("redirect_uri", h.deps.OIDC.RedirectURL)
	values.Set("client_id", h.deps.OIDC.ClientID)
	if h.deps.OIDC.ClientSecret != "" {
		values.Set("client_secret", h.deps.OIDC.ClientSecret)
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, discovery.TokenEndpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return oidcTokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return oidcTokenResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return oidcTokenResponse{}, errors.New("token endpoint failed")
	}
	var token oidcTokenResponse
	if err := json.NewDecoder(response.Body).Decode(&token); err != nil {
		return oidcTokenResponse{}, err
	}
	if token.AccessToken == "" {
		return oidcTokenResponse{}, errors.New("missing access token")
	}
	return token, nil
}

func (h Handler) loadOIDCUserinfo(r *http.Request, discovery oidcDiscovery, accessToken string) (oidcUserinfo, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, discovery.UserinfoEndpoint, nil)
	if err != nil {
		return oidcUserinfo{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return oidcUserinfo{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return oidcUserinfo{}, errors.New("userinfo endpoint failed")
	}
	var userinfo oidcUserinfo
	if err := json.NewDecoder(response.Body).Decode(&userinfo); err != nil {
		return oidcUserinfo{}, err
	}
	return userinfo, nil
}

func (h Handler) validateOIDCState(r *http.Request) error {
	cookie, err := r.Cookie(oidcStateCookieName)
	if err != nil {
		return err
	}
	if cookie.Value == "" || cookie.Value != r.URL.Query().Get("state") {
		return errors.New("oidc state mismatch")
	}
	return nil
}

func (h Handler) postLoginRedirect() string {
	if h.deps.OIDC.PostLoginRedirect != "" {
		return h.deps.OIDC.PostLoginRedirect
	}
	return "/"
}

func setOIDCState(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookieName,
		Value:    state,
		Path:     "/auth/oidc",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearOIDCState(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookieName,
		Value:    "",
		Path:     "/auth/oidc",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func randomState() (string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes[:]), nil
}

func (u oidcUserinfo) displayName() string {
	for _, value := range []string{u.Name, u.PreferredUsername, u.Email} {
		if value != "" {
			return value
		}
	}
	return ""
}
