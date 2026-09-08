package factory

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dolthub/cli/internal/authflow"
	"github.com/dolthub/cli/internal/credentials"

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
	for _, tt := range []struct {
		name, host, override, want string
		set, nilLookup             bool
	}{
		{name: "production unset", host: productionHost, want: productionOAuthClientID},
		{name: "production nil lookup", host: productionHost, nilLookup: true, want: productionOAuthClientID},
		{name: "production empty", host: productionHost, set: true, want: productionOAuthClientID},
		{name: "production whitespace", host: productionHost, set: true, override: "  ", want: productionOAuthClientID},
		{name: "production case", host: "WWW.DOLTHUB.COM", want: productionOAuthClientID},
		{name: "production override", host: productionHost, set: true, override: " override-client ", want: "override-client"},
		{name: "custom override", host: "example.test", set: true, override: " custom-client ", want: "custom-client"},
		{name: "custom unset", host: "example.test"},
		{name: "custom whitespace", host: "example.test", set: true, override: "  "},
		{name: "custom nil lookup", host: "example.test", nilLookup: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(name string) (string, bool) { return tt.override, name == OAuthClientIDEnv && tt.set }
			if tt.nilLookup {
				lookup = nil
			}
			got, err := oauthClientID(tt.host, lookup)
			if tt.want == "" {
				if err == nil || !strings.Contains(err.Error(), tt.host) || !strings.Contains(err.Error(), OAuthClientIDEnv) {
					t.Fatalf("missing client ID error = %v", err)
				}
			} else if err != nil || got != tt.want {
				t.Fatalf("client ID = %q, error = %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestProductionOAuthAuthorization(t *testing.T) {
	client, err := oauthClient("test", config.DefaultHost, nil)
	if err != nil {
		t.Fatal(err)
	}
	authorization, err := client.NewAuthorization(authflow.DefaultCallbackURL)
	if err != nil {
		t.Fatal(err)
	}
	if authorization.URL.Scheme != "https" || authorization.URL.Host != "www.dolthub.com" || authorization.URL.Path != "/oauth/authorize" {
		t.Fatalf("authorization URL = %s", authorization.URL)
	}
	for key, want := range map[string]string{
		"client_id":             "dhoci.v1.nbdj3mjpe8sdes2c7i1i9ul3l2jm091h24o2s7f462aj3b55qsv0",
		"redirect_uri":          "http://localhost:53682/callback",
		"scope":                 "api_read_write",
		"code_challenge_method": "S256",
		"response_type":         "code",
	} {
		if got := authorization.URL.Query().Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if authorization.URL.Query().Get("code_challenge") == "" {
		t.Fatal("missing PKCE challenge")
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

func TestProductionTokenRefresh(t *testing.T) {
	for _, automatic := range []bool{false, true} {
		name := "direct"
		if automatic {
			name = "API automatic"
		}
		t.Run(name, func(t *testing.T) {
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			refreshes, reads := 0, 0
			http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
				body := ""
				switch r.URL.String() {
				case "https://www.dolthub.com/api/oauth/access_token":
					refreshes++
					if err := r.ParseForm(); err != nil {
						t.Fatal(err)
					}
					if r.Method != http.MethodPost || r.PostForm.Get("client_id") != productionOAuthClientID || r.PostForm.Get("grant_type") != "refresh_token" || r.PostForm.Get("refresh_token") != "old-refresh" || r.PostForm.Has("client_secret") {
						t.Fatalf("unexpected refresh request: %s %v", r.Method, r.PostForm)
					}
					body = `{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":3600}`
				case "https://www.dolthub.com/api/v2/user":
					reads++
					if r.Header.Get("Authorization") != "Bearer new-access" {
						t.Fatal("API request did not use refreshed access token")
					}
					body = `{"data":{"username":"test-user"}}`
				default:
					t.Fatalf("unexpected URL: %s", r.URL)
				}
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})
			streams, _, _, _ := iostreams.NewTest()
			f := New("test", streams)
			cfg := config.NewMemory()
			cfg.SetActiveUser(productionHost, "test-user")
			f.Config = func() (config.Config, error) { return cfg, nil }
			f.LookupEnv = func(string) (string, bool) { return "", false }
			f.Credentials = credentials.NewMemoryStore()
			if automatic {
				err := credentials.SetOAuthToken(f.Credentials, productionHost, "test-user", credentials.OAuthToken{AccessToken: "old-access", RefreshToken: "old-refresh", ExpiresAt: time.Now().Add(-time.Hour)})
				if err != nil {
					t.Fatal(err)
				}
				client, err := f.APIClientForHost(productionHost)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := client.CurrentUser(context.Background()); err != nil {
					t.Fatal(err)
				}
				stored, err := credentials.GetStoredOAuthToken(f.Credentials, productionHost, "test-user")
				if err != nil {
					t.Fatal(err)
				}
				if stored.Token.RefreshToken != "new-refresh" {
					t.Fatal("rotated refresh token was not persisted")
				}
				if reads != 1 {
					t.Fatalf("API reads = %d, want 1", reads)
				}
			} else {
				token, err := f.RefreshToken(context.Background(), "old-refresh")
				if err != nil {
					t.Fatal(err)
				}
				if token.AccessToken != "new-access" {
					t.Fatal("unexpected refreshed access token")
				}
			}
			if refreshes != 1 {
				t.Fatalf("refreshes = %d, want 1", refreshes)
			}
		})
	}
}
