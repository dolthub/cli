package view

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/credentials"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

type fakeClient struct {
	id        string
	operation dolthub.Operation
}

func (f *fakeClient) GetOperation(_ context.Context, id string) (dolthub.Operation, error) {
	f.id = id
	return f.operation, nil
}

func authenticatedOptions(t *testing.T, operation dolthub.Operation) (*Options, *fakeClient, *strings.Builder) {
	t.Helper()
	streams, _, _, _ := iostreams.NewTest()
	client := &fakeClient{operation: operation}
	builder := &strings.Builder{}
	streams.Out = builder
	return &Options{
		IO:        streams,
		Config:    func() (config.Config, error) { return config.NewMemory(), nil },
		LookupEnv: func(key string) (string, bool) { return "token", key == "DH_TOKEN" },
		ID:        "owners/acme/repos/r/jobs/1",
		client:    client,
	}, client, builder
}

func TestViewFailedJobSucceedsAndRendersError(t *testing.T) {
	op := dolthub.Operation{ID: "repositoryOwners/dolthub/repositories/people/jobs/716a6b3f-4bd4-432e-b7ae-87bead012a3f", Type: dolthub.OperationFork, Status: dolthub.OperationFailed, CreatedAt: time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC), Error: &dolthub.OperationError{Code: "FAILED", Title: "Failed"}}
	opts, client, out := authenticatedOptions(t, op)
	if err := viewRun(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if client.id != opts.ID || !strings.Contains(out.String(), "ID\t716a6b3f-4bd4-432e-b7ae-87bead012a3f\n") || !strings.Contains(out.String(), "failed") || !strings.Contains(out.String(), "FAILED") {
		t.Fatalf("ID = %q, output = %q", client.id, out.String())
	}
}

func TestViewRequiresAuthentication(t *testing.T) {
	streams, _, _, _ := iostreams.NewTest()
	opts := &Options{IO: streams, Config: func() (config.Config, error) { return config.NewMemory(), nil }, client: &fakeClient{}}
	err := viewRun(context.Background(), opts)
	var authErr *cmdutil.AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("error = %T %v", err, err)
	}
}

func TestViewAcceptsStoredAuthentication(t *testing.T) {
	cfg := config.NewMemory()
	cfg.SetActiveUser(config.DefaultHost, "alice")
	store := credentials.NewMemoryStore()
	if err := credentials.SetOAuthToken(store, config.DefaultHost, "alice", credentials.OAuthToken{AccessToken: "token"}); err != nil {
		t.Fatal(err)
	}
	streams, _, _, _ := iostreams.NewTest()
	opts := &Options{IO: streams, Config: func() (config.Config, error) { return cfg, nil }, Credentials: store, ID: "id", client: &fakeClient{operation: dolthub.Operation{ID: "id"}}}
	if err := viewRun(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
}

func TestViewStructuredDynamicResult(t *testing.T) {
	opts, _, out := authenticatedOptions(t, dolthub.Operation{ID: "repositoryOwners/dolthub/repositories/people/jobs/716a6b3f-4bd4-432e-b7ae-87bead012a3f", Result: []byte(`{"database":{"owner":"o"}}`)})
	cmd := NewCmdView(&cmdutil.Factory{}, func(_ context.Context, parsed *Options) error {
		parsed.IO, parsed.Config, parsed.LookupEnv, parsed.client = opts.IO, opts.Config, opts.LookupEnv, opts.client
		return viewRun(context.Background(), parsed)
	})
	cmd.SetArgs([]string{"owners/acme/jobs/1", "--json", "id,result"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"id":"716a6b3f-4bd4-432e-b7ae-87bead012a3f"`) || !strings.Contains(out.String(), `"result":{"database":{"owner":"o"}}`) {
		t.Fatalf("output = %q", out.String())
	}
}

func TestViewValidatesID(t *testing.T) {
	cmd := NewCmdView(&cmdutil.Factory{}, func(context.Context, *Options) error { return nil })
	cmd.SetArgs([]string{" "})
	err := cmd.Execute()
	var flagErr *cmdutil.FlagError
	if !errors.As(err, &flagErr) {
		t.Fatalf("error = %T %v", err, err)
	}
}
