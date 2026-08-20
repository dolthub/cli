package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dolthub/cli/internal/credentials"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/httptransport"
)

const (
	DefaultScope        = "api_read_write"
	codeChallengeMethod = "S256"
	maxTokenResponse    = 1 << 20
)

// Authorization is one short-lived browser authorization attempt.
type Authorization struct {
	URL          *url.URL
	State        string
	CodeVerifier string
}

// Client implements the network portion of DoltHub's public-client OAuth
// authorization-code flow. Browser and callback handling live above it.
type Client struct {
	httpClient *http.Client
	origin     *url.URL
	clientID   string
	scope      string
	random     io.Reader
	now        func() time.Time
}

func NewClient(httpClient *http.Client, origin *url.URL, clientID string) (*Client, error) {
	if httpClient == nil {
		return nil, errors.New("oauth http client is required")
	}
	if err := validateOrigin(origin); err != nil {
		return nil, err
	}
	if strings.TrimSpace(clientID) == "" {
		return nil, errors.New("oauth client ID is required")
	}
	copyOrigin := *origin
	return &Client{httpClient: httpClient, origin: &copyOrigin, clientID: clientID, scope: DefaultScope, random: rand.Reader, now: time.Now}, nil
}

func validateOrigin(origin *url.URL) error {
	if origin == nil || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" {
		return errors.New("oauth origin must be an HTTPS origin")
	}
	if origin.Path != "" && origin.Path != "/" {
		return errors.New("oauth origin must not contain a path")
	}
	return nil
}

// NewAuthorization creates the state, PKCE verifier, and authorization URL
// for one callback attempt.
func (c *Client) NewAuthorization(redirectURI string) (Authorization, error) {
	if err := validateRedirectURI(redirectURI); err != nil {
		return Authorization{}, err
	}
	state, err := randomBase64URL(c.random, 32)
	if err != nil {
		return Authorization{}, errors.New("generate oauth state")
	}
	verifier, err := randomBase64URL(c.random, 32)
	if err != nil {
		return Authorization{}, errors.New("generate PKCE verifier")
	}
	challenge := S256Challenge(verifier)
	authorizeURL := c.endpoint("/oauth/authorize")
	query := authorizeURL.Query()
	query.Set("response_type", "code")
	query.Set("client_id", c.clientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", c.scope)
	query.Set("state", state)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", codeChallengeMethod)
	authorizeURL.RawQuery = query.Encode()
	return Authorization{URL: authorizeURL, State: state, CodeVerifier: verifier}, nil
}

func randomBase64URL(source io.Reader, size int) (string, error) {
	b := make([]byte, size)
	if _, err := io.ReadFull(source, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// S256Challenge derives the RFC 7636 S256 challenge for verifier.
func S256Challenge(verifier string) string {
	digest := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func validateRedirectURI(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" || u.User != nil || u.Fragment != "" {
		return errors.New("oauth redirect URI is invalid")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("oauth redirect URI has an unsupported scheme")
	}
	return nil
}

func (c *Client) ExchangeCode(ctx context.Context, code, redirectURI, verifier string) (credentials.OAuthToken, error) {
	if code == "" || verifier == "" {
		return credentials.OAuthToken{}, errors.New("authorization code and PKCE verifier are required")
	}
	if err := validateRedirectURI(redirectURI); err != nil {
		return credentials.OAuthToken{}, err
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {c.clientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}
	return c.exchange(ctx, form)
}

// Refresh exchanges and rotates a public client's refresh token. Its
// signature satisfies credentials.RefreshFunc.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (credentials.OAuthToken, error) {
	if refreshToken == "" {
		return credentials.OAuthToken{}, errors.New("refresh token is required")
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {c.clientID},
		"refresh_token": {refreshToken},
	}
	return c.exchange(ctx, form)
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func (c *Client) exchange(ctx context.Context, form url.Values) (credentials.OAuthToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/api/oauth/access_token").String(), strings.NewReader(form.Encode()))
	if err != nil {
		return credentials.OAuthToken{}, errors.New("create oauth token request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return credentials.OAuthToken{}, ctx.Err()
		}
		return credentials.OAuthToken{}, errors.New("oauth token request failed")
	}
	defer resp.Body.Close()
	var body tokenResponse
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxTokenResponse))
	if err := decoder.Decode(&body); err != nil {
		return credentials.OAuthToken{}, &ExchangeError{Status: resp.StatusCode, Code: "invalid_response"}
	}
	if body.Error != "" || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		code := safeErrorCode(body.Error)
		if code == "" {
			code = "http_error"
		}
		return credentials.OAuthToken{}, &ExchangeError{Status: resp.StatusCode, Code: code}
	}
	if body.ExpiresIn <= 0 {
		return credentials.OAuthToken{}, &ExchangeError{Status: resp.StatusCode, Code: "invalid_response"}
	}
	token := credentials.OAuthToken{
		AccessToken:  body.AccessToken,
		RefreshToken: body.RefreshToken,
		TokenType:    body.TokenType,
		ExpiresAt:    c.now().Add(time.Duration(body.ExpiresIn) * time.Second),
	}
	if err := token.Validate(); err != nil || token.RefreshToken == "" || !strings.EqualFold(token.TokenType, "Bearer") {
		return credentials.OAuthToken{}, &ExchangeError{Status: resp.StatusCode, Code: "invalid_response"}
	}
	return token, nil
}

// CurrentUsername validates accessToken against DoltHub API v2 and returns
// the authenticated username.
func (c *Client) CurrentUsername(ctx context.Context, accessToken string) (string, error) {
	transport, err := httptransport.NewAuthenticated(c.httpClient.Transport, "oauth", c.origin.Hostname(), accessToken)
	if err != nil {
		return "", err
	}
	apiBase := c.endpoint("/api/v2/")
	api, err := dolthub.NewClient(&http.Client{Transport: transport}, apiBase)
	if err != nil {
		return "", err
	}
	user, err := api.CurrentUser(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errors.New("validate oauth identity")
	}
	if user.Username == "" {
		return "", errors.New("validate oauth identity: response has no username")
	}
	return user.Username, nil
}

func safeErrorCode(code string) string {
	if code == "" {
		return ""
	}
	if len(code) > 64 {
		return "oauth_error"
	}
	for _, r := range code {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' && r != '-' {
			return "oauth_error"
		}
	}
	return code
}

func (c *Client) endpoint(path string) *url.URL {
	u := *c.origin
	u.Path = path
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return &u
}

// ExchangeError is a safe OAuth token-endpoint failure. The server's free-form
// description is intentionally not retained because it may echo credentials.
type ExchangeError struct {
	Status int
	Code   string
}

func (e *ExchangeError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("oauth token exchange failed: %s (HTTP %d)", e.Code, e.Status)
	}
	return "oauth token exchange failed: " + e.Code
}
