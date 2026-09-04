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

type fakeClient struct{ request dolthub.CreateTagRequest }

func (f *fakeClient) CreateTag(_ context.Context, _, _ string, r dolthub.CreateTagRequest) (dolthub.Tag, error) {
	f.request = r
	return dolthub.Tag{Name: r.Name, CommitSHA: "abc", Message: value(r.Message)}, nil
}
func value(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
func resolve(context.Context, string) (repository.Repository, error) {
	return repository.Repository{Host: "h", Owner: "o", Name: "d"}, nil
}
func TestCreate(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{}
	o := &Options{IO: io, ResolveRepository: resolve, Name: "v1", FromCommit: "abc", Message: "first", client: c}
	if err := createRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if c.request.From.Commit != "abc" || c.request.Message == nil || !strings.Contains(out.String(), "v1\tabc") {
		t.Fatalf("request=%#v output=%q", c.request, out.String())
	}
}
func TestValidation(t *testing.T) {
	io, _, _, _ := iostreams.NewTest()
	f := &cmdutil.Factory{IO: io}
	cmd := NewCmdCreate(f, func(context.Context, *Options) error { return nil })
	cmd.SetArgs([]string{"v1"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("missing source accepted")
	}
}
