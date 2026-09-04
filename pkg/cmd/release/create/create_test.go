package create

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

type fakeClient struct{ request dolthub.CreateReleaseRequest }

func (f *fakeClient) CreateRelease(_ context.Context, _, _ string, r dolthub.CreateReleaseRequest) (dolthub.Release, error) {
	f.request = r
	return dolthub.Release{Tag: r.Tag, Title: r.Title, CommitSHA: r.CommitSHA}, nil
}
func resolve(context.Context, string) (repository.Repository, error) {
	return repository.Repository{Host: "h", Owner: "o", Name: "d"}, nil
}
func TestCreateWithNotesFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(file, []byte("notes\n"), 0600); err != nil {
		t.Fatal(err)
	}
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{}
	o := &Options{IO: io, ResolveRepository: resolve, Tag: "v1", Title: "First", Target: "abc", NotesFile: file, CreateTag: true, client: c}
	if err := createRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if c.request.Description == nil || *c.request.Description != "notes\n" || !c.request.CreateTagIfNotExists || !strings.Contains(out.String(), "v1\tFirst\tabc") {
		t.Fatalf("request=%#v output=%q", c.request, out.String())
	}
}
func TestValidation(t *testing.T) {
	io, _, _, _ := iostreams.NewTest()
	f := &cmdutil.Factory{IO: io}
	for _, args := range [][]string{{"v1"}, {"v1", "--title", "x"}, {"v1", "--title", "x", "--target", "a", "--notes", "x", "--notes-file", "f"}} {
		{
			cmd := NewCmdCreate(f, func(context.Context, *Options) error { return nil })
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("args %v accepted", args)
			}
		}
	}
}
