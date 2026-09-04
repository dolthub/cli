package comment

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
	request dolthub.CreatePullCommentRequest
}

func (f *fakeClient) CreatePullComment(_ context.Context, _, _ string, n int64, r dolthub.CreatePullCommentRequest) (dolthub.PullComment, error) {
	f.number = n
	f.request = r
	return dolthub.PullComment{CommentID: "c1", Author: "alice", Body: r.Body}, nil
}
func resolve(context.Context, string) (repository.Repository, error) {
	return repository.Repository{Host: "h", Owner: "o", Name: "d"}, nil
}
func TestComment(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{}
	o := &Options{IO: io, ResolveRepository: resolve, Number: 42, Body: "hello", client: c}
	if err := commentRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if c.number != 42 || c.request.Body != "hello" || !strings.Contains(out.String(), "alice") {
		t.Fatalf("number=%d request=%#v output=%q", c.number, c.request, out.String())
	}
}
func TestValidation(t *testing.T) {
	io, _, _, _ := iostreams.NewTest()
	f := &cmdutil.Factory{IO: io}
	for _, args := range [][]string{{"0", "--body", "x"}, {"1"}, {"1", "--body", "x", "--body-file", "f"}} {
		{
			cmd := NewCmdComment(f, func(context.Context, *Options) error { return nil })
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("args %v accepted", args)
			}
		}
	}
}
