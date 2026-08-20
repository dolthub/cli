package status

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/credentials"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestStatusEnvironmentCredential(t *testing.T) {
	streams, _, out, errOut := iostreams.NewTest()
	cfg := config.NewMemory()
	secret := "fake-secret-token"
	transport := roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer "+secret {
			t.Fatalf("auth=%q", r.Header.Get("Authorization"))
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"data":{"username":"alice"}}`)), Request: r}, nil
	})
	opts := &Options{IO: streams, Config: func() (config.Config, error) { return cfg, nil }, Credentials: credentials.NewMemoryStore(), LookupEnv: func(string) (string, bool) { return secret, true }, AppVersion: "test", BaseTransport: transport}
	if err := statusRun(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "DH_TOKEN") || strings.Contains(out.String()+errOut.String(), secret) {
		t.Fatal("bad output")
	}
}

func TestStatusStoredCredential(t *testing.T) {
	streams, _, out, _ := iostreams.NewTest()
	cfg := config.NewMemory()
	cfg.SetActiveUser(config.DefaultHost, "alice")
	store := credentials.NewMemoryStore()
	if err := credentials.SetOAuthToken(store, config.DefaultHost, "alice", credentials.OAuthToken{AccessToken: "stored-token", TokenType: "Bearer"}); err != nil {
		t.Fatal(err)
	}
	transport := roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer stored-token" {
			t.Fatal("wrong token")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"data":{"username":"alice"}}`)), Request: r}, nil
	})
	opts := &Options{IO: streams, Config: func() (config.Config, error) { return cfg, nil }, Credentials: store, LookupEnv: func(string) (string, bool) { return "", false }, AppVersion: "test", BaseTransport: transport}
	if err := statusRun(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "keyring") {
		t.Fatal(out.String())
	}
}

func TestStatusReportsFileCredential(t *testing.T) {
	streams, _, out, _ := iostreams.NewTest()
	cfg := config.NewMemory()
	cfg.SetActiveUser(config.DefaultHost, "alice")
	store := credentials.NewFallbackStore(credentials.NewMemoryStore(), credentials.NewFileStore(filepath.Join(t.TempDir(), "credentials.json")))
	if err := credentials.SetOAuthTokenAt(store, credentials.SourceFile, config.DefaultHost, "alice", credentials.OAuthToken{AccessToken: "stored-token", TokenType: "Bearer"}); err != nil {
		t.Fatal(err)
	}
	transport := roundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"data":{"username":"alice"}}`)), Request: r}, nil
	})
	opts := &Options{IO: streams, Config: func() (config.Config, error) { return cfg, nil }, Credentials: store, LookupEnv: func(string) (string, bool) { return "", false }, AppVersion: "test", BaseTransport: transport}
	if err := statusRun(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "credential file") {
		t.Fatal(out.String())
	}
}

func TestStatusRefreshesStoredCredential(t *testing.T) {
	streams, _, out, _ := iostreams.NewTest()
	cfg := config.NewMemory()
	cfg.SetActiveUser(config.DefaultHost, "alice")
	store := credentials.NewMemoryStore()
	now := time.Now()
	if err := credentials.SetOAuthToken(store, config.DefaultHost, "alice", credentials.OAuthToken{AccessToken: "expired", RefreshToken: "refresh", TokenType: "Bearer", ExpiresAt: now.Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	base := roundTrip(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("Authorization"); got != "Bearer refreshed" {
			t.Fatalf("authorization = %q", got)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"data":{"username":"alice"}}`)), Request: r}, nil
	})
	opts := &Options{IO: streams, Config: func() (config.Config, error) { return cfg, nil }, Credentials: store, LookupEnv: func(string) (string, bool) { return "", false }, AppVersion: "test", BaseTransport: base, RefreshToken: func(context.Context, string) (credentials.OAuthToken, error) {
		return credentials.OAuthToken{AccessToken: "refreshed", RefreshToken: "rotated", TokenType: "Bearer", ExpiresAt: now.Add(time.Hour)}, nil
	}}
	if err := statusRun(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "keyring") {
		t.Fatal(out.String())
	}
	stored, err := credentials.GetOAuthToken(store, config.DefaultHost, "alice")
	if err != nil || stored.RefreshToken != "rotated" {
		t.Fatalf("stored = %#v, err = %v", stored, err)
	}
}

func TestStatusMissingAndInvalid(t *testing.T) {
	for _, tt := range []struct {
		name      string
		setup     func(*config.Memory, *credentials.MemoryStore)
		transport http.RoundTripper
	}{
		{name: "missing", setup: func(*config.Memory, *credentials.MemoryStore) {}},
		{name: "invalid", setup: func(c *config.Memory, s *credentials.MemoryStore) {
			c.SetActiveUser(config.DefaultHost, "alice")
			_ = credentials.SetOAuthToken(s, config.DefaultHost, "alice", credentials.OAuthToken{AccessToken: "bad", TokenType: "Bearer"})
		}, transport: roundTrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 401, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"title":"Unauthorized"}`)), Request: r}, nil
		})},
	} {
		t.Run(tt.name, func(t *testing.T) {
			streams, _, _, _ := iostreams.NewTest()
			cfg := config.NewMemory()
			store := credentials.NewMemoryStore()
			tt.setup(cfg, store)
			opts := &Options{IO: streams, Config: func() (config.Config, error) { return cfg, nil }, Credentials: store, LookupEnv: func(string) (string, bool) { return "", false }, AppVersion: "test", BaseTransport: tt.transport}
			err := statusRun(context.Background(), opts)
			var authErr *cmdutil.AuthError
			if !errors.As(err, &authErr) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
