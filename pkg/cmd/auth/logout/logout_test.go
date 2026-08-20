package logout

import (
	"context"
	"errors"
	"testing"

	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/credentials"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

func TestLogout(t *testing.T) {
	streams, _, out, _ := iostreams.NewTest()
	cfg := config.NewMemory()
	cfg.SetActiveUser(config.DefaultHost, "alice")
	store := credentials.NewMemoryStore()
	_ = store.Set(config.DefaultHost, "alice", "fake")
	opts := &Options{IO: streams, Config: func() (config.Config, error) { return cfg, nil }, Credentials: store, LookupEnv: func(string) (string, bool) { return "", false }, Yes: true}
	if err := logoutRun(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.ActiveUser(config.DefaultHost); ok {
		t.Fatal("active user retained")
	}
	if _, err := store.Get(config.DefaultHost, "alice"); !errors.Is(err, credentials.ErrNotFound) {
		t.Fatal(err)
	}
	if out.String() == "" {
		t.Fatal("missing output")
	}
}

func TestNewCmdLogoutParsesOptions(t *testing.T) {
	streams, _, _, _ := iostreams.NewTest()
	var got *Options
	f := &cmdutil.Factory{IO: streams, LookupEnv: func(string) (string, bool) { return "", false }}
	cmd := NewCmdLogout(f, func(_ context.Context, o *Options) error { got = o; return nil })
	cmd.SetArgs([]string{"--hostname", "dev.example", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Host != "dev.example" || !got.Yes {
		t.Fatalf("options=%#v", got)
	}
}

func TestLogoutProtectsEnvironmentToken(t *testing.T) {
	opts := &Options{LookupEnv: func(string) (string, bool) { return "fake-token", true }}
	if err := logoutRun(context.Background(), opts); err == nil {
		t.Fatal("expected error")
	}
}

func TestLogoutRestoresCredentialOnConfigFailure(t *testing.T) {
	streams, _, _, _ := iostreams.NewTest()
	cfg := config.NewMemory()
	cfg.SetActiveUser(config.DefaultHost, "alice")
	cfg.WriteErr = errors.New("disk failed")
	store := credentials.NewMemoryStore()
	if err := store.Set(config.DefaultHost, "alice", "old-token"); err != nil {
		t.Fatal(err)
	}
	opts := &Options{IO: streams, Config: func() (config.Config, error) { return cfg, nil }, Credentials: store, LookupEnv: func(string) (string, bool) { return "", false }, Yes: true}
	if err := logoutRun(context.Background(), opts); err == nil {
		t.Fatal("expected error")
	}
	if got, err := store.Get(config.DefaultHost, "alice"); err != nil || got != "old-token" {
		t.Fatalf("credential = %q, err = %v", got, err)
	}
	if user, ok := cfg.ActiveUser(config.DefaultHost); !ok || user != "alice" {
		t.Fatalf("active user = %q, %v", user, ok)
	}
}

type rejectingPrompt struct{}

func (rejectingPrompt) Confirm(string, bool) (bool, error) { return false, nil }
func TestLogoutCancellation(t *testing.T) {
	streams, _, _, _ := iostreams.NewTest()
	streams.SetStdinTTY(true)
	cfg := config.NewMemory()
	cfg.SetActiveUser(config.DefaultHost, "alice")
	opts := &Options{IO: streams, Config: func() (config.Config, error) { return cfg, nil }, Credentials: credentials.NewMemoryStore(), Prompter: rejectingPrompt{}, LookupEnv: func(string) (string, bool) { return "", false }}
	err := logoutRun(context.Background(), opts)
	var cancel *cmdutil.CancelError
	if !errors.As(err, &cancel) {
		t.Fatalf("err=%v", err)
	}
}
