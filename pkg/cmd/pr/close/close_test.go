package close

import (
	"context"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"strings"
	"testing"
)

type fakeClient struct {
	number  int64
	request dolthub.UpdatePullRequest
}

func (f *fakeClient) UpdatePull(_ context.Context, _, _ string, n int64, r dolthub.UpdatePullRequest) (dolthub.Pull, error) {
	f.number = n
	f.request = r
	return dolthub.Pull{PullNumber: n, Title: "Fix", State: *r.State}, nil
}
func resolve(context.Context, string) (repository.Repository, error) {
	return repository.Repository{Host: "h", Owner: "o", Name: "d"}, nil
}
func TestClose(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{}
	o := &Options{IO: io, ResolveRepository: resolve, Number: 42, client: c}
	if err := closeRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if c.request.State == nil || *c.request.State != dolthub.PullStateClosed || !strings.Contains(out.String(), "42\tFix\tclosed") {
		t.Fatalf("request=%#v output=%q", c.request, out.String())
	}
}
func TestCloseJSONAndValidation(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{}
	f := &cmdutil.Factory{IO: io, ResolveRepository: resolve}
	cmd := NewCmdClose(f, func(ctx context.Context, o *Options) error { o.client = c; return closeRun(ctx, o) })
	cmd.SetArgs([]string{"2", "--json", "pull_number,state"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"pull_number":2`) || !strings.Contains(out.String(), `"state":"closed"`) {
		t.Fatal(out.String())
	}
	cmd = NewCmdClose(f, func(context.Context, *Options) error { return nil })
	cmd.SetArgs([]string{"0"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("zero accepted")
	}
}
