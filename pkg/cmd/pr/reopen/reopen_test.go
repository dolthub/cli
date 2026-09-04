package reopen

import (
	"context"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"strings"
	"testing"
)

type fakeClient struct{ request dolthub.UpdatePullRequest }

func (f *fakeClient) UpdatePull(_ context.Context, _, _ string, n int64, r dolthub.UpdatePullRequest) (dolthub.Pull, error) {
	f.request = r
	return dolthub.Pull{PullNumber: n, Title: "Fix", State: *r.State}, nil
}
func resolve(context.Context, string) (repository.Repository, error) {
	return repository.Repository{Host: "h", Owner: "o", Name: "d"}, nil
}
func TestReopen(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{}
	o := &Options{IO: io, ResolveRepository: resolve, Number: 42, client: c}
	if err := reopenRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if c.request.State == nil || *c.request.State != dolthub.PullStateOpen || !strings.Contains(out.String(), "42\tFix\topen") {
		t.Fatalf("request=%#v output=%q", c.request, out.String())
	}
}
func TestReopenJSON(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{}
	f := &cmdutil.Factory{IO: io, ResolveRepository: resolve}
	cmd := NewCmdReopen(f, func(ctx context.Context, o *Options) error { o.client = c; return reopenRun(ctx, o) })
	cmd.SetArgs([]string{"2", "--json", "state"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.String() != `{"state":"open"}`+"\n" {
		t.Fatal(out.String())
	}
}
