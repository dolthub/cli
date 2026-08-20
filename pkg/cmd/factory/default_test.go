package factory

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/credentials"
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

func TestOAuthClientIDUsesDevelopmentEnvironment(t *testing.T) {
	lookup := func(name string) (string, bool) {
		if name == OAuthClientIDEnv {
			return " dev-public-client ", true
		}
		return "", false
	}
	clientID, err := oauthClientID(lookup)
	if err != nil {
		t.Fatal(err)
	}
	if clientID != "dev-public-client" {
		t.Fatalf("client ID = %q", clientID)
	}
	if _, err := oauthClientID(func(string) (string, bool) { return "", false }); err == nil || !strings.Contains(err.Error(), OAuthClientIDEnv) {
		t.Fatalf("missing client ID error = %v", err)
	}
}

func TestProductionAuthenticatorReportsMissingDevelopmentClientID(t *testing.T) {
	streams, _, _, _ := iostreams.NewTest()
	f := New("test", streams)
	f.LookupEnv = func(string) (string, bool) { return "", false }
	_, err := f.Authenticator.Login(context.Background(), "dev.dolthub.com")
	if err == nil || !strings.Contains(err.Error(), OAuthClientIDEnv) {
		t.Fatalf("login error = %v", err)
	}
}

func TestOAuthClientUsesConfiguredDevelopmentHost(t *testing.T) {
	client, err := oauthClient("test", "dev.dolthub.com", func(name string) (string, bool) {
		return "dev-public-client", name == OAuthClientIDEnv
	})
	if err != nil {
		t.Fatal(err)
	}
	authorization, err := client.NewAuthorization("http://localhost:53682/callback")
	if err != nil {
		t.Fatal(err)
	}
	if authorization.URL.Host != "dev.dolthub.com" || authorization.URL.Path != "/oauth/authorize" {
		t.Fatalf("authorization URL = %s", authorization.URL)
	}
	if authorization.URL.Query().Get("client_id") != "dev-public-client" {
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
	cfg.SetHost("dev.dolthub.com")
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
	if token.AccessToken != "new-access" || requestURL != "https://dev.dolthub.com/api/oauth/access_token" {
		t.Fatalf("token = %#v, URL = %q", token, requestURL)
	}
	if form.Get("client_id") != "dev-public-client" || form.Get("refresh_token") != "old-refresh-secret" || form.Get("client_secret") != "" {
		t.Fatalf("refresh form = %v", form)
	}
}

func TestHTTPClientRefreshesAndPersistsStoredCredential(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	var apiAuthorization string
	http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
		body := "{}"
		if r.URL.Path == "/api/oauth/access_token" {
			body = `{"access_token":"rotated-access","refresh_token":"rotated-refresh","token_type":"Bearer","expires_in":3600}`
		} else {
			apiAuthorization = r.Header.Get("Authorization")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})

	streams, _, _, _ := iostreams.NewTest()
	f := New("test", streams)
	cfg := config.NewMemory()
	cfg.SetHost("dev.dolthub.com")
	cfg.SetActiveUser("dev.dolthub.com", "alice")
	store := credentials.NewMemoryStore()
	if err := credentials.SetOAuthToken(store, "dev.dolthub.com", "alice", credentials.OAuthToken{AccessToken: "expired-access", RefreshToken: "old-refresh", TokenType: "Bearer", ExpiresAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	f.Config = func() (config.Config, error) { return cfg, nil }
	f.Credentials = store
	f.LookupEnv = func(name string) (string, bool) {
		if name == OAuthClientIDEnv {
			return "dev-public-client", true
		}
		return "", false
	}
	client, err := f.HTTPClient()
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://dev.dolthub.com/api/v2/user", nil)
	if _, err := client.Do(req); err != nil {
		t.Fatal(err)
	}
	if apiAuthorization != "Bearer rotated-access" {
		t.Fatalf("API authorization = %q", apiAuthorization)
	}
	rotated, err := credentials.GetOAuthToken(store, "dev.dolthub.com", "alice")
	if err != nil || rotated.RefreshToken != "rotated-refresh" {
		t.Fatalf("stored token = %#v, error = %v", rotated, err)
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
