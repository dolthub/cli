package merge

import (
	"context"
	"errors"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/operationwaiter"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"strings"
	"testing"
)

type fakeClient struct {
	number    int64
	operation dolthub.Operation
}

func (f *fakeClient) MergePull(_ context.Context, _, _ string, n int64) (dolthub.OperationRef, error) {
	f.number = n
	return dolthub.OperationRef{ID: "1", Href: "https://h/api/v2/operations/1"}, nil
}
func (f *fakeClient) GetOperation(context.Context, string) (dolthub.Operation, error) {
	return dolthub.Operation{}, nil
}
func (f *fakeClient) GetOperationURL(context.Context, string) (dolthub.Operation, error) {
	return f.operation, nil
}
func resolve(context.Context, string) (repository.Repository, error) {
	return repository.Repository{Host: "h", Owner: "o", Name: "d"}, nil
}
func TestMergeNoWait(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{}
	o := &Options{IO: io, ResolveRepository: resolve, Number: 42, NoWait: true, client: c}
	if err := mergeRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if c.number != 42 || !strings.Contains(out.String(), "1\thttps://h") {
		t.Fatalf("number=%d output=%q", c.number, out.String())
	}
}
func TestMergeWaitFailure(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{}
	failed := dolthub.Operation{ID: "1", Status: dolthub.OperationFailed}
	o := &Options{IO: io, ResolveRepository: resolve, Number: 42, client: c, wait: func(context.Context, dolthub.OperationRef) (dolthub.Operation, error) {
		return failed, &operationwaiter.FailedError{Operation: failed}
	}}
	err := mergeRun(context.Background(), o)
	var target *operationwaiter.FailedError
	if !errors.As(err, &target) || !strings.Contains(out.String(), "failed") {
		t.Fatalf("error=%v output=%q", err, out.String())
	}
}
func TestMergeReportsWaitStatusInTTY(t *testing.T) {
	io, _, _, errOut := iostreams.NewTest()
	io.SetStderrTTY(true)
	c := &fakeClient{operation: dolthub.Operation{ID: "1", Status: dolthub.OperationSucceeded}}
	o := &Options{IO: io, ResolveRepository: resolve, Number: 42, client: c}
	if err := mergeRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if got := errOut.String(); !strings.Contains(got, "Waiting for job 1: succeeded") {
		t.Fatalf("stderr=%q", got)
	}
}
func TestMergeValidationAndJSON(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{}
	f := &cmdutil.Factory{IO: io, ResolveRepository: resolve}
	cmd := NewCmdMerge(f, func(ctx context.Context, o *Options) error { o.client = c; return mergeRun(ctx, o) })
	cmd.SetArgs([]string{"2", "--no-wait", "--json", "id,href"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"href":"https://h/api/v2/operations/1"`) {
		t.Fatal(out.String())
	}
	cmd = NewCmdMerge(f, func(context.Context, *Options) error { return nil })
	cmd.SetArgs([]string{"0"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("zero accepted")
	}
}
