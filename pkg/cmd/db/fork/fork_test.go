package fork

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
	users, creates int
	request        dolthub.CreateForkRequest
	operation      dolthub.Operation
}

func (f *fakeClient) CurrentUser(context.Context) (dolthub.User, error) {
	f.users++
	return dolthub.User{Username: "alice"}, nil
}
func (f *fakeClient) CreateFork(_ context.Context, _, _ string, r dolthub.CreateForkRequest) (dolthub.OperationRef, error) {
	f.creates++
	f.request = r
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
func TestForkNoWaitUsesCurrentUser(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{}
	o := &Options{IO: io, ResolveRepository: resolve, NoWait: true, client: c}
	if err := forkRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if c.users != 1 || c.request.Owner != "alice" || !strings.Contains(out.String(), "1\thttps://h") {
		t.Fatalf("users=%d request=%#v output=%q", c.users, c.request, out.String())
	}
}
func TestForkWaitFailureIsRendered(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{}
	failed := dolthub.Operation{ID: "1", Status: dolthub.OperationFailed}
	o := &Options{IO: io, ResolveRepository: resolve, Organization: "org", client: c, wait: func(context.Context, dolthub.OperationRef) (dolthub.Operation, error) {
		return failed, &operationwaiter.FailedError{Operation: failed}
	}}
	err := forkRun(context.Background(), o)
	var target *operationwaiter.FailedError
	if !errors.As(err, &target) || c.users != 0 || !strings.Contains(out.String(), "failed") {
		t.Fatalf("error=%v users=%d output=%q", err, c.users, out.String())
	}
}
func TestForkReportsWaitStatusInTTY(t *testing.T) {
	io, _, _, errOut := iostreams.NewTest()
	io.SetStderrTTY(true)
	c := &fakeClient{operation: dolthub.Operation{ID: "1", Status: dolthub.OperationSucceeded}}
	o := &Options{IO: io, ResolveRepository: resolve, Organization: "org", client: c}
	if err := forkRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if got := errOut.String(); !strings.Contains(got, "Waiting for operation 1: succeeded") {
		t.Fatalf("stderr=%q", got)
	}
}
func TestForkValidation(t *testing.T) {
	io, _, _, _ := iostreams.NewTest()
	f := &cmdutil.Factory{IO: io}
	cmd := NewCmdFork(f, func(context.Context, *Options) error { return nil })
	cmd.SetArgs([]string{"o/d", "--db", "x/y"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("argument and flag accepted")
	}
}
