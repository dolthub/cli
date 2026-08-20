package login

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dolthub/cli/internal/authflow"
	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/credentials"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

type fakeAuth struct {
	result authflow.LoginResult
	err    error
}

func TestLoginWarnsWhenKeyringFallsBackToFile(t *testing.T) {
	streams, _, out, errOut := iostreams.NewTest()
	cfg := config.NewMemory()
	keyring := credentials.NewMemoryStore()
	keyring.Err = errors.New("keyring unavailable")
	path := filepath.Join(t.TempDir(), "credentials.json")
	store := credentials.NewFallbackStore(keyring, credentials.NewFileStore(path))
	opts := &Options{IO: streams, Config: func() (config.Config, error) { return cfg, nil }, Credentials: store, Authenticator: fakeAuth{result: authflow.LoginResult{Host: config.DefaultHost, Username: "alice", Credential: credentials.OAuthToken{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer"}}}, LookupEnv: noEnv}
	if err := loginRun(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "saved unencrypted") || !strings.Contains(errOut.String(), path) {
		t.Fatalf("warning = %q", errOut.String())
	}
	if strings.Contains(out.String()+errOut.String(), "access") || strings.Contains(out.String()+errOut.String(), "refresh") {
		t.Fatal("credential leaked")
	}
	stored, err := credentials.GetStoredOAuthToken(store, config.DefaultHost, "alice")
	if err != nil || stored.Source != credentials.SourceFile {
		t.Fatalf("stored = %#v, error = %v", stored, err)
	}
}

func (f fakeAuth) Login(context.Context, string) (authflow.LoginResult, error) {
	return f.result, f.err
}

func TestNewCmdLoginParsesHostname(t *testing.T) {
	streams, _, _, _ := iostreams.NewTest()
	var got *Options
	f := &cmdutil.Factory{IO: streams, LookupEnv: noEnv}
	cmd := NewCmdLogin(f, func(_ context.Context, o *Options) error { got = o; return nil })
	cmd.SetArgs([]string{"--hostname", "dev.example"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Host != "dev.example" {
		t.Fatalf("options=%#v", got)
	}
}

func TestLoginPersistsValidatedResult(t *testing.T) {
	streams, _, out, errOut := iostreams.NewTest()
	cfg := config.NewMemory()
	store := credentials.NewMemoryStore()
	secret := "fake-secret-token"
	opts := &Options{IO: streams, Config: func() (config.Config, error) { return cfg, nil }, Credentials: store, Authenticator: fakeAuth{result: authflow.LoginResult{Host: config.DefaultHost, Username: "alice", Credential: credentials.OAuthToken{AccessToken: secret, RefreshToken: "refresh-token", TokenType: "Bearer"}}}, LookupEnv: noEnv}
	if err := loginRun(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if user, _ := cfg.ActiveUser(config.DefaultHost); user != "alice" {
		t.Fatal(user)
	}
	if got, err := credentials.GetOAuthToken(store, config.DefaultHost, "alice"); err != nil || got.AccessToken != secret {
		t.Fatal("credential not stored")
	}
	if strings.Contains(out.String()+errOut.String(), secret) {
		t.Fatal("token leaked")
	}
}

func TestLoginRollsBackCredentialOnConfigFailure(t *testing.T) {
	streams, _, _, _ := iostreams.NewTest()
	cfg := config.NewMemory()
	cfg.WriteErr = errors.New("disk failed")
	store := credentials.NewMemoryStore()
	opts := &Options{IO: streams, Config: func() (config.Config, error) { return cfg, nil }, Credentials: store, Authenticator: fakeAuth{result: authflow.LoginResult{Host: config.DefaultHost, Username: "alice", Credential: credentials.OAuthToken{AccessToken: "fake-token"}}}, LookupEnv: noEnv}
	if err := loginRun(context.Background(), opts); err == nil {
		t.Fatal("expected error")
	}
	if _, err := store.Get(config.DefaultHost, "alice"); !errors.Is(err, credentials.ErrNotFound) {
		t.Fatal("credential was not rolled back")
	}
	if _, ok := cfg.ActiveUser(config.DefaultHost); ok {
		t.Fatal("active user was not rolled back")
	}
}

func TestLoginRestoresExistingCredentialOnConfigFailure(t *testing.T) {
	streams, _, _, _ := iostreams.NewTest()
	cfg := config.NewMemory()
	cfg.SetActiveUser(config.DefaultHost, "alice")
	cfg.WriteErr = errors.New("disk failed")
	store := credentials.NewMemoryStore()
	if err := store.Set(config.DefaultHost, "alice", "old-token"); err != nil {
		t.Fatal(err)
	}
	opts := &Options{IO: streams, Config: func() (config.Config, error) { return cfg, nil }, Credentials: store, Authenticator: fakeAuth{result: authflow.LoginResult{Host: config.DefaultHost, Username: "alice", Credential: credentials.OAuthToken{AccessToken: "new-token"}}}, LookupEnv: noEnv}
	if err := loginRun(context.Background(), opts); err == nil {
		t.Fatal("expected error")
	}
	if got, err := store.Get(config.DefaultHost, "alice"); err != nil || got != "old-token" {
		t.Fatalf("credential = %q, err = %v", got, err)
	}
	if user, ok := cfg.ActiveUser(config.DefaultHost); !ok || user != "alice" {
		t.Fatalf("active user = %q, %v", user, ok)
	}
}

func TestLoginRejectsMismatchedHost(t *testing.T) {
	cfg := config.NewMemory()
	opts := &Options{Config: func() (config.Config, error) { return cfg, nil }, Credentials: credentials.NewMemoryStore(), Authenticator: fakeAuth{result: authflow.LoginResult{Host: "evil.example", Username: "alice", Credential: credentials.OAuthToken{AccessToken: "fake"}}}, LookupEnv: noEnv}
	if err := loginRun(context.Background(), opts); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoginPropagatesCancellation(t *testing.T) {
	canceled := context.Canceled
	cfg := config.NewMemory()
	opts := &Options{Config: func() (config.Config, error) { return cfg, nil }, Credentials: credentials.NewMemoryStore(), Authenticator: fakeAuth{err: canceled}, LookupEnv: noEnv}
	if err := loginRun(context.Background(), opts); !errors.Is(err, canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestLoginRefusesEnvironmentToken(t *testing.T) {
	opts := &Options{LookupEnv: func(string) (string, bool) { return "fake-token", true }}
	if err := loginRun(context.Background(), opts); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoginReplacesUserOnSameHost(t *testing.T) {
	streams, _, _, _ := iostreams.NewTest()
	cfg := config.NewMemory()
	cfg.SetActiveUser(config.DefaultHost, "old")
	store := credentials.NewMemoryStore()
	_ = store.Set(config.DefaultHost, "old", "old-token")
	opts := &Options{IO: streams, Config: func() (config.Config, error) { return cfg, nil }, Credentials: store, Authenticator: fakeAuth{result: authflow.LoginResult{Host: config.DefaultHost, Username: "new", Credential: credentials.OAuthToken{AccessToken: "new-token"}}}, LookupEnv: noEnv}
	if err := loginRun(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(config.DefaultHost, "old"); !errors.Is(err, credentials.ErrNotFound) {
		t.Fatal("old credential retained")
	}
	if user, _ := cfg.ActiveUser(config.DefaultHost); user != "new" {
		t.Fatalf("user=%q", user)
	}
}

func noEnv(string) (string, bool) { return "", false }
