package httptransport

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAuthenticatedTransportTrustBoundary(t *testing.T) {
	const token = "fake-secret-token"
	var requests []*http.Request
	base := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("{}")), Header: http.Header{}, Request: r}, nil
	})
	transport, err := NewAuthenticated(base, "1.2.3", "api.example.com", token)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: transport}
	for _, rawURL := range []string{"https://api.example.com/api/v2/user", "https://evil.example/api/v2/user", "http://api.example.com/api/v2/user"} {
		req, _ := http.NewRequest(http.MethodGet, rawURL, nil)
		if _, err := client.Do(req); err != nil {
			t.Fatal(err)
		}
	}
	if got := requests[0].Header.Get("Authorization"); got != "Bearer "+token {
		t.Fatalf("auth=%q", got)
	}
	if requests[1].Header.Get("Authorization") != "" || requests[2].Header.Get("Authorization") != "" {
		t.Fatal("credentials escaped trusted HTTPS host")
	}
	for _, r := range requests {
		if r.Header.Get("User-Agent") != "dh/1.2.3" {
			t.Fatalf("user agent=%q", r.Header.Get("User-Agent"))
		}
	}
}

func TestAuthenticatedTransportDoesNotMutateRequest(t *testing.T) {
	base := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("{}")), Header: http.Header{}, Request: r}, nil
	})
	tr, _ := NewAuthenticated(base, "dev", "example.com", "secret")
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	_, _ = tr.RoundTrip(req)
	if req.Header.Get("Authorization") != "" {
		t.Fatal("original request mutated")
	}
}
