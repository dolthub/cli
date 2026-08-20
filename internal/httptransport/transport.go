package httptransport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// New returns a transport that adds the dh user agent to every request.
func New(base http.RoundTripper, version string) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		clone := req.Clone(req.Context())
		clone.Header = req.Header.Clone()
		clone.Header.Set("User-Agent", "dh/"+version)
		return base.RoundTrip(clone)
	})
}

// NewAuthenticated adds a bearer token only for the exact trusted hostname.
func NewAuthenticated(base http.RoundTripper, version, trustedHost, token string) (http.RoundTripper, error) {
	if token == "" {
		return nil, errors.New("authentication token is empty")
	}
	return NewAuthenticatedTokenSource(base, version, trustedHost, staticAccessToken(token))
}

// AccessTokenSource returns the bearer token to attach to one request.
type AccessTokenSource interface {
	AccessToken(context.Context) (string, error)
}

type staticAccessToken string

func (s staticAccessToken) AccessToken(context.Context) (string, error) { return string(s), nil }

// NewAuthenticatedTokenSource adds a bearer token from a dynamic source only
// for the exact trusted hostname.
func NewAuthenticatedTokenSource(base http.RoundTripper, version, trustedHost string, source AccessTokenSource) (http.RoundTripper, error) {
	trustedHost = strings.TrimSpace(strings.ToLower(trustedHost))
	if trustedHost == "" || strings.ContainsAny(trustedHost, "/:?#@") {
		return nil, errors.New("trusted host must be a hostname")
	}
	if source == nil {
		return nil, errors.New("authentication token source is nil")
	}
	base = New(base, version)
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" || !strings.EqualFold(req.URL.Hostname(), trustedHost) {
			return base.RoundTrip(req)
		}
		token, err := source.AccessToken(req.Context())
		if err != nil {
			return nil, fmt.Errorf("load authentication token: %w", err)
		}
		if token == "" {
			return nil, errors.New("load authentication token: token is empty")
		}
		clone := req.Clone(req.Context())
		clone.Header = req.Header.Clone()
		clone.Header.Set("Authorization", "Bearer "+token)
		return base.RoundTrip(clone)
	}), nil
}
