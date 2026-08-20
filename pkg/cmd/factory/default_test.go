package factory

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/pkg/iostreams"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNew(t *testing.T) {
	streams, _, _, _ := iostreams.NewTest()
	f := New("1.2.3", streams)

	if got, want := f.AppVersion, "1.2.3"; got != want {
		t.Fatalf("AppVersion = %q, want %q", got, want)
	}
	if f.IO != streams {
		t.Fatal("IO does not contain the supplied streams")
	}
}

func TestAPIBaseURL(t *testing.T) {
	base, err := apiBaseURL("dev.dolthub.com")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := base.String(), "https://dev.dolthub.com/api/v2/"; got != want {
		t.Fatalf("base = %q, want %q", got, want)
	}
	for _, host := range []string{"", "https://example.com", "user@example.com", "example.com/path", "example.com:8443"} {
		if _, err := apiBaseURL(host); err == nil {
			t.Errorf("apiBaseURL(%q) unexpectedly succeeded", host)
		}
	}
}

func TestHTTPClientUsesEnvironmentTokenWithoutActiveUser(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	var authorization string
	http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
		authorization = r.Header.Get("Authorization")
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("{}")), Request: r}, nil
	})
	streams, _, _, _ := iostreams.NewTest()
	f := New("test", streams)
	cfg := config.NewMemory()
	f.Config = func() (config.Config, error) { return cfg, nil }
	f.LookupEnv = func(name string) (string, bool) {
		if name == "DH_TOKEN" {
			return "environment-secret", true
		}
		return "", false
	}
	client, err := f.HTTPClient()
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://www.dolthub.com/api/v2/user", nil)
	if _, err := client.Do(req); err != nil {
		t.Fatal(err)
	}
	if authorization != "Bearer environment-secret" {
		t.Fatalf("authorization = %q", authorization)
	}
}
