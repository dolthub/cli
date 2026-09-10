package watch

import (
	"context"
	"errors"
	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/operationwaiter"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"strings"
	"testing"
	"time"
)

func authenticatedConfig() config.Config {
	c := config.NewMemory()
	c.Users[config.DefaultHost] = "alice"
	return c
}
func TestWatchRendersFailedJobAndReturnsFailure(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	failed := &operationwaiter.FailedError{Operation: dolthub.Operation{ID: "1", Status: dolthub.OperationFailed, Error: &dolthub.OperationError{Title: "bad"}}}
	o := &Options{IO: io, Config: func() (config.Config, error) { return authenticatedConfig(), nil }, LookupEnv: func(k string) (string, bool) { return "token", k == "DH_TOKEN" }, ID: "1", wait: func(context.Context, string) (dolthub.Operation, error) { return failed.Operation, failed }}
	err := watchRun(context.Background(), o)
	if !errors.As(err, &failed) || !strings.Contains(out.String(), "failed") {
		t.Fatalf("error=%v output=%q", err, out.String())
	}
}
func TestWatchStructuredSuccess(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	f := &cmdutil.Factory{IO: io}
	cmd := NewCmdWatch(f, func(_ context.Context, o *Options) error {
		o.Config = func() (config.Config, error) { return authenticatedConfig(), nil }
		o.LookupEnv = func(string) (string, bool) { return "token", true }
		o.wait = func(context.Context, string) (dolthub.Operation, error) {
			return dolthub.Operation{ID: "repositoryOwners/dolthub/repositories/people/jobs/716a6b3f-4bd4-432e-b7ae-87bead012a3f", Status: dolthub.OperationSucceeded}, nil
		}
		return watchRun(context.Background(), o)
	})
	cmd.SetArgs([]string{"a/b", "--json", "id,status"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"id":"716a6b3f-4bd4-432e-b7ae-87bead012a3f"`) || !strings.Contains(out.String(), `"status":"succeeded"`) {
		t.Fatal(out.String())
	}
}
func TestWatchValidation(t *testing.T) {
	io, _, _, _ := iostreams.NewTest()
	f := &cmdutil.Factory{IO: io}
	for _, args := range [][]string{{}, {"1", "--interval", "0s"}, {"1", "--interval", "-1s"}} {
		{
			cmd := NewCmdWatch(f, func(context.Context, *Options) error { return nil })
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("args %v accepted", args)
			}
		}
	}
}

type pollingClient struct{ calls int }

func (c *pollingClient) GetOperation(context.Context, string) (dolthub.Operation, error) {
	c.calls++
	status := dolthub.OperationRunning
	if c.calls == 2 {
		status = dolthub.OperationSucceeded
	}
	return dolthub.Operation{ID: "job/1", Status: status}, nil
}
func (c *pollingClient) GetOperationURL(context.Context, string) (dolthub.Operation, error) {
	return dolthub.Operation{}, errors.New("unexpected")
}
func TestWatchReportsPollingStatusInTTY(t *testing.T) {
	io, _, _, errOut := iostreams.NewTest()
	io.SetStderrTTY(true)
	client := &pollingClient{}
	o := &Options{IO: io, Config: func() (config.Config, error) { return authenticatedConfig(), nil }, LookupEnv: func(string) (string, bool) { return "token", true }, ID: "job/1", Interval: time.Millisecond, client: client}
	if err := watchRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if got := errOut.String(); !strings.Contains(got, "running") || !strings.Contains(got, "succeeded") {
		t.Fatalf("output=%q", got)
	}
}
