package create

import (
	"context"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"strings"
	"testing"
)

type fakeClient struct{ request dolthub.CreateBranchRequest }

func (f *fakeClient) CreateBranch(_ context.Context, _, _ string, r dolthub.CreateBranchRequest) (dolthub.Branch, error) {
	f.request = r
	return dolthub.Branch{Name: r.Name, HeadCommitSHA: "abc"}, nil
}
func resolve(context.Context, string) (repository.Repository, error) {
	return repository.Repository{Host: "h", Owner: "o", Name: "d"}, nil
}
func TestCreate(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{}
	o := &Options{IO: io, ResolveRepository: resolve, Name: "new", FromBranch: "main", client: c}
	if err := createRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if c.request.From.Branch != "main" || !strings.Contains(out.String(), "new\tabc") {
		t.Fatalf("request=%#v output=%q", c.request, out.String())
	}
}
func TestValidation(t *testing.T) {
	io, _, _, _ := iostreams.NewTest()
	f := &cmdutil.Factory{IO: io}
	for _, args := range [][]string{{"new"}, {"new", "--from-branch", "main", "--from-commit", "abc"}} {
		{
			cmd := NewCmdCreate(f, func(context.Context, *Options) error { return nil })
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("args %v accepted", args)
			}
		}
	}
}
