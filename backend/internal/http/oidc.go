package httpapi

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
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
	JWKSURI               string `json:"jwks_uri"`
}

type oidcTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
}

type oidcJWKS struct {
	Keys []oidcJWK `json:"keys"`
}

type oidcJWK struct {
	KeyType string `json:"kty"`
	Use     string `json:"use"`
	KeyID   string `json:"kid"`
	Alg     string `json:"alg"`
	N       string `json:"n"`
	E       string `json:"e"`
}

type oidcIDTokenHeader struct {
	Alg   string `json:"alg"`
	KeyID string `json:"kid"`
}

type oidcIDTokenClaims struct {
	Issuer   string          `json:"iss"`
	Audience json.RawMessage `json:"aud"`
	Subject  string          `json:"sub"`
	Expires  int64           `json:"exp"`
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
	idTokenSubject, err := h.validateOIDCIDToken(r, discovery, token.IDToken)
	if err != nil {
		http.Error(w, "OIDC id token validation failed", http.StatusBadGateway)
		return
	}
	userinfo, err := h.loadOIDCUserinfo(r, discovery, token.AccessToken)
	if err != nil {
		http.Error(w, "OIDC userinfo failed", http.StatusBadGateway)
		return
	}
	displayName := userinfo.displayName()
	if displayName == "" || userinfo.Subject == "" || userinfo.Subject != idTokenSubject {
		http.Error(w, "OIDC userinfo is incomplete", http.StatusBadGateway)
		return
	}
	user, err := h.deps.Store.UpsertUser(r.Context(), "oidc:"+userinfo.Subject, displayName)
	if err != nil {
		http.Error(w, "failed to upsert OIDC user", http.StatusInternalServerError)
		return
	}
	h.setSession(w, user.ID)
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
	if discovery.AuthorizationEndpoint == "" || discovery.TokenEndpoint == "" || discovery.UserinfoEndpoint == "" || discovery.JWKSURI == "" {
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
	if token.IDToken == "" {
		return oidcTokenResponse{}, errors.New("missing id token")
	}
	return token, nil
}

func (h Handler) validateOIDCIDToken(r *http.Request, discovery oidcDiscovery, token string) (string, error) {
	header, claims, signingInput, signature, err := parseIDToken(token)
	if err != nil {
		return "", err
	}
	if header.Alg != "RS256" || header.KeyID == "" {
		return "", errors.New("unsupported id token header")
	}
	key, err := h.oidcSigningKey(r, discovery, header.KeyID)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], signature); err != nil {
		return "", err
	}
	if claims.Issuer != h.deps.OIDC.IssuerURL {
		return "", errors.New("id token issuer mismatch")
	}
	if !claims.hasAudience(h.deps.OIDC.ClientID) {
		return "", errors.New("id token audience mismatch")
	}
	if claims.Subject == "" {
		return "", errors.New("id token subject is empty")
	}
	if claims.Expires <= time.Now().Unix() {
		return "", errors.New("id token expired")
	}
	return claims.Subject, nil
}

func parseIDToken(token string) (oidcIDTokenHeader, oidcIDTokenClaims, string, []byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return oidcIDTokenHeader{}, oidcIDTokenClaims{}, "", nil, errors.New("invalid id token")
	}
	headerBody, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return oidcIDTokenHeader{}, oidcIDTokenClaims{}, "", nil, err
	}
	claimsBody, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return oidcIDTokenHeader{}, oidcIDTokenClaims{}, "", nil, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return oidcIDTokenHeader{}, oidcIDTokenClaims{}, "", nil, err
	}
	var header oidcIDTokenHeader
	if err := json.Unmarshal(headerBody, &header); err != nil {
		return oidcIDTokenHeader{}, oidcIDTokenClaims{}, "", nil, err
	}
	var claims oidcIDTokenClaims
	if err := json.Unmarshal(claimsBody, &claims); err != nil {
		return oidcIDTokenHeader{}, oidcIDTokenClaims{}, "", nil, err
	}
	return header, claims, parts[0] + "." + parts[1], signature, nil
}

func (h Handler) oidcSigningKey(r *http.Request, discovery oidcDiscovery, keyID string) (*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, discovery.JWKSURI, nil)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errors.New("jwks endpoint failed")
	}
	var jwks oidcJWKS
	if err := json.NewDecoder(response.Body).Decode(&jwks); err != nil {
		return nil, err
	}
	for _, key := range jwks.Keys {
		if key.KeyID == keyID && key.KeyType == "RSA" && key.N != "" && key.E != "" {
			return rsaPublicKey(key)
		}
	}
	return nil, errors.New("matching jwk not found")
}

func rsaPublicKey(key oidcJWK) (*rsa.PublicKey, error) {
	modulusBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, err
	}
	exponentBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, err
	}
	exponent := 0
	for _, value := range exponentBytes {
		exponent = exponent<<8 + int(value)
	}
	if exponent == 0 {
		return nil, errors.New("invalid RSA exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(modulusBytes), E: exponent}, nil
}

func (c oidcIDTokenClaims) hasAudience(clientID string) bool {
	var single string
	if err := json.Unmarshal(c.Audience, &single); err == nil {
		return single == clientID
	}
	var many []string
	if err := json.Unmarshal(c.Audience, &many); err == nil {
		for _, value := range many {
			if value == clientID {
				return true
			}
		}
	}
	return false
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
