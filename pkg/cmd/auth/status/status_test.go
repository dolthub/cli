package status

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

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
	_ = store.Set(config.DefaultHost, "alice", "stored-token")
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
	if !strings.Contains(out.String(), "credential store") {
		t.Fatal(out.String())
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
			_ = s.Set(config.DefaultHost, "alice", "bad")
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
