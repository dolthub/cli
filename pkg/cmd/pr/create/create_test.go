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

type fakeClient struct{ request dolthub.CreatePullRequest }

func (f *fakeClient) CreatePull(_ context.Context, _, _ string, r dolthub.CreatePullRequest) (dolthub.Pull, error) {
	f.request = r
	return dolthub.Pull{PullNumber: 7, Title: r.Title, State: dolthub.PullStateOpen, FromBranch: r.FromBranch, ToBranch: r.ToBranch}, nil
}
func resolve(context.Context, string) (repository.Repository, error) {
	return repository.Repository{Host: "h", Owner: "acme", Name: "widgets"}, nil
}
func TestCreateCrossFork(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{}
	o := &Options{IO: io, ResolveRepository: resolve, Title: "Fix", Body: "body", Head: "alice/fork:work", Base: "main", client: c}
	if err := createRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if c.request.FromBranch.Database.Owner != "alice" || c.request.ToBranch.Database.Owner != "acme" || !strings.Contains(out.String(), "7\tFix\topen") {
		t.Fatalf("request=%#v output=%q", c.request, out.String())
	}
}
func TestSameDatabaseHead(t *testing.T) {
	r := repository.Repository{Owner: "o", Name: "d"}
	head, err := parseHead("work", r)
	if err != nil || head.Database.Owner != "o" || head.BranchName != "work" {
		t.Fatalf("head=%#v err=%v", head, err)
	}
}
func TestValidation(t *testing.T) {
	io, _, _, _ := iostreams.NewTest()
	f := &cmdutil.Factory{IO: io}
	for _, args := range [][]string{{}, {"--title", "x", "--head", "h", "--base", "b", "--body", "x", "--body-file", "f"}} {
		{
			cmd := NewCmdCreate(f, func(context.Context, *Options) error { return nil })
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("args %v accepted", args)
			}
		}
	}
	for _, bad := range []string{"a/b/c:work", "a/b:", "a:b:c"} {
		if _, err := parseHead(bad, repository.Repository{}); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
}
