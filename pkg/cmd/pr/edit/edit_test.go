package edit

import (
	"context"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeClient struct{ request dolthub.UpdatePullRequest }

func (f *fakeClient) UpdatePull(_ context.Context, _, _ string, n int64, r dolthub.UpdatePullRequest) (dolthub.Pull, error) {
	f.request = r
	title := "old"
	description := "old"
	if r.Title != nil {
		title = *r.Title
	}
	if r.Description != nil {
		description = *r.Description
	}
	return dolthub.Pull{PullNumber: n, Title: title, Description: description, State: dolthub.PullStateOpen}, nil
}
func resolve(context.Context, string) (repository.Repository, error) {
	return repository.Repository{Host: "h", Owner: "o", Name: "d"}, nil
}
func TestEditPreservesFieldPresence(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{}
	f := &cmdutil.Factory{IO: io, ResolveRepository: resolve}
	cmd := NewCmdEdit(f, func(ctx context.Context, o *Options) error { o.client = c; return editRun(ctx, o) })
	cmd.SetArgs([]string{"2", "--title="})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if c.request.Title == nil || *c.request.Title != "" || c.request.Description != nil || !strings.Contains(out.String(), "2\t\topen") {
		t.Fatalf("request=%#v output=%q", c.request, out.String())
	}
}
func TestEditEmptyBodyFileClearsDescription(t *testing.T) {
	file := filepath.Join(t.TempDir(), "body")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	io, _, _, _ := iostreams.NewTest()
	c := &fakeClient{}
	o := &Options{IO: io, ResolveRepository: resolve, Number: 3, BodyFile: file, BodyFileSet: true, client: c}
	if err := editRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if c.request.Description == nil || *c.request.Description != "" {
		t.Fatalf("request=%#v", c.request)
	}
}
func TestEditValidation(t *testing.T) {
	io, _, _, _ := iostreams.NewTest()
	f := &cmdutil.Factory{IO: io}
	for _, args := range [][]string{{"1"}, {"1", "--body", "x", "--body-file", "f"}, {"bad", "--title", "x"}} {
		{
			cmd := NewCmdEdit(f, func(context.Context, *Options) error { return nil })
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("args %v accepted", args)
			}
		}
	}
}
