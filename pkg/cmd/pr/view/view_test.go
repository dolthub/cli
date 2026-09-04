package view

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

type fakeClient struct {
	pull                   dolthub.Pull
	comments               []dolthub.PullComment
	getCalls, commentCalls int
}

func (f *fakeClient) GetPull(context.Context, string, string, int64) (dolthub.Pull, error) {
	f.getCalls++
	return f.pull, nil
}
func (f *fakeClient) ListPullComments(context.Context, string, string, int64) ([]dolthub.PullComment, error) {
	f.commentCalls++
	return f.comments, nil
}

type fakeBrowser struct {
	url string
	err error
}

func (f *fakeBrowser) Browse(u string) error { f.url = u; return f.err }
func resolver(context.Context, string) (repository.Repository, error) {
	return repository.Repository{Host: "example.test", Owner: "acme/west", Name: "widgets"}, nil
}

func samplePull() dolthub.Pull {
	return dolthub.Pull{PullNumber: 42, Title: "Fix it", Description: "Body", State: dolthub.PullStateOpen, Creator: "alice", CreatedAt: time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC), FromBranch: dolthub.BranchRef{Database: dolthub.DatabaseRef{Owner: "alice", Name: "fork"}, BranchName: "work"}, ToBranch: dolthub.BranchRef{Database: dolthub.DatabaseRef{Owner: "acme", Name: "widgets"}, BranchName: "main"}}
}

func TestViewPullAndOptionalComments(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{pull: samplePull(), comments: []dolthub.PullComment{{Author: "bob", Body: "Looks good", CreatedAt: time.Date(2026, 9, 4, 13, 0, 0, 0, time.UTC)}}}
	o := &Options{IO: io, ResolveRepository: resolver, Number: 42, Comments: true, client: c}
	if err := viewRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if c.getCalls != 1 || c.commentCalls != 1 || !strings.Contains(out.String(), "alice/fork:work") || !strings.Contains(out.String(), "Looks good") {
		t.Fatalf("calls=%d/%d output=%q", c.getCalls, c.commentCalls, out.String())
	}
}

func TestViewPullJSONWithComments(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	f := &cmdutil.Factory{IO: io, ResolveRepository: resolver}
	c := &fakeClient{pull: samplePull(), comments: []dolthub.PullComment{{CommentID: "c1", Author: "bob"}}}
	cmd := NewCmdView(f, func(ctx context.Context, o *Options) error { o.client = c; return viewRun(ctx, o) })
	cmd.SetArgs([]string{"42", "--comments", "--json", "pull_number,comments"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"pull_number":42`) || !strings.Contains(out.String(), `"comment_id":"c1"`) {
		t.Fatal(out.String())
	}
}

func TestViewPullWebSkipsAPI(t *testing.T) {
	b := &fakeBrowser{}
	want := errors.New("browser")
	b.err = want
	err := viewRun(context.Background(), &Options{ResolveRepository: resolver, Browser: b, Number: 42, Web: true})
	if !errors.Is(err, want) || b.url != "https://example.test/repositories/acme%2Fwest/widgets/pulls/42" {
		t.Fatalf("url=%q err=%v", b.url, err)
	}
}

func TestViewPullValidation(t *testing.T) {
	io, _, _, _ := iostreams.NewTest()
	f := &cmdutil.Factory{IO: io}
	for _, args := range [][]string{{}, {"zero"}, {"0"}, {"1", "--web", "--comments"}} {
		cmd := NewCmdView(f, func(context.Context, *Options) error { return nil })
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("args %v: expected error", args)
		}
	}
}
