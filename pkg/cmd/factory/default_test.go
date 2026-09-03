package factory

import (
	"context"
	"io"
	"net/http"
	"net/url"
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
	if f.Authenticator == nil || f.RefreshToken == nil {
		t.Fatal("production authentication dependencies are not configured")
	}
	if f.ResolveRepository == nil {
		t.Fatal("repository resolver is not configured")
	}
}

func TestAPIBaseURL(t *testing.T) {
	base, err := apiBaseURL(productionHost)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := base.String(), "https://www.dolthub.com/api/v2/"; got != want {
		t.Fatalf("base = %q, want %q", got, want)
	}
	for _, host := range []string{"", "https://example.com", "user@example.com", "example.com/path", "example.com:8443"} {
		if _, err := apiBaseURL(host); err == nil {
			t.Errorf("apiBaseURL(%q) unexpectedly succeeded", host)
		}
	}
}

func TestOAuthClientIDSelection(t *testing.T) {
	lookup := func(name string) (string, bool) {
		if name == OAuthClientIDEnv {
			return " override-public-client ", true
		}
		return "", false
	}
	clientID, err := oauthClientID("example.test", lookup)
	if err != nil {
		t.Fatal(err)
	}
	if clientID != "override-public-client" {
		t.Fatalf("client ID = %q", clientID)
	}

	if _, err := oauthClientID("www.dolthub.com", func(string) (string, bool) { return "", false }); err == nil || !strings.Contains(err.Error(), "www.dolthub.com") {
		t.Fatalf("missing client ID error = %v", err)
	}
}

func TestProductionAuthenticatorReportsMissingProductionClientID(t *testing.T) {
	streams, _, _, _ := iostreams.NewTest()
	f := New("test", streams)
	f.LookupEnv = func(string) (string, bool) { return "", false }
	_, err := f.Authenticator.Login(context.Background(), "www.dolthub.com")
	if err == nil || !strings.Contains(err.Error(), "www.dolthub.com") {
		t.Fatalf("login error = %v", err)
	}
}

func TestOAuthClientUsesConfiguredHostAndClientID(t *testing.T) {
	client, err := oauthClient("test", "example.test", func(name string) (string, bool) {
		return "configured-public-client", name == OAuthClientIDEnv
	})
	if err != nil {
		t.Fatal(err)
	}
	authorization, err := client.NewAuthorization("http://localhost:53682/callback")
	if err != nil {
		t.Fatal(err)
	}
	if authorization.URL.Host != "example.test" || authorization.URL.Path != "/oauth/authorize" {
		t.Fatalf("authorization URL = %s", authorization.URL)
	}
	if authorization.URL.Query().Get("client_id") != "configured-public-client" {
		t.Fatalf("authorization query = %v", authorization.URL.Query())
	}
}

func TestRefreshTokenUsesConfiguredHostAndPublicClient(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	var requestURL string
	var form url.Values
	http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
		requestURL = r.URL.String()
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		form = r.PostForm
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":3600}`)), Request: r}, nil
	})
	streams, _, _, _ := iostreams.NewTest()
	f := New("test", streams)
	cfg := config.NewMemory()
	cfg.SetHost("example.test")
	f.Config = func() (config.Config, error) { return cfg, nil }
	f.LookupEnv = func(name string) (string, bool) {
		if name == OAuthClientIDEnv {
			return "dev-public-client", true
		}
		return "", false
	}
	token, err := f.RefreshToken(context.Background(), "old-refresh-secret")
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "new-access" || requestURL != "https://example.test/api/oauth/access_token" {
		t.Fatalf("token = %#v, URL = %q", token, requestURL)
	}
	if form.Get("client_id") != "dev-public-client" || form.Get("refresh_token") != "old-refresh-secret" || form.Get("client_secret") != "" {
		t.Fatalf("refresh form = %v", form)
	}
}
