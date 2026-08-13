package httptransport

import (
	"errors"
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
	trustedHost = strings.TrimSpace(strings.ToLower(trustedHost))
	if trustedHost == "" || strings.ContainsAny(trustedHost, "/:?#@") {
		return nil, errors.New("trusted host must be a hostname")
	}
	if token == "" {
		return nil, errors.New("authentication token is empty")
	}
	base = New(base, version)
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" || !strings.EqualFold(req.URL.Hostname(), trustedHost) {
			return base.RoundTrip(req)
		}
		clone := req.Clone(req.Context())
		clone.Header = req.Header.Clone()
		clone.Header.Set("Authorization", "Bearer "+token)
		return base.RoundTrip(clone)
	}), nil
}
