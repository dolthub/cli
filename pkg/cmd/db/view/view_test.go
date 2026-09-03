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
	database dolthub.Database
	forks    []dolthub.DatabaseRef
	gets     int
	lists    int
}

func (f *fakeClient) GetDatabase(context.Context, string, string) (dolthub.Database, error) {
	f.gets++
	return f.database, nil
}
func (f *fakeClient) ListForks(context.Context, string, string) ([]dolthub.DatabaseRef, error) {
	f.lists++
	return f.forks, nil
}

type fakeBrowser struct{ url string }

func (f *fakeBrowser) Browse(value string) error { f.url = value; return nil }

func TestViewHumanAndForks(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	client := &fakeClient{database: dolthub.Database{Owner: "o", Name: "r", Visibility: dolthub.VisibilityPublic, LastWriteAt: &now}, forks: []dolthub.DatabaseRef{{Owner: "f", Name: "r"}}}
	streams, _, out, _ := iostreams.NewTest()
	opts := &Options{IO: streams, ResolveRepository: fixedRepo, Forks: true, client: client}
	if err := viewRun(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if client.gets != 1 || client.lists != 1 || !strings.Contains(out.String(), "o/r") || !strings.Contains(out.String(), `"owner":"f"`) {
		t.Fatalf("gets=%d lists=%d output=%q", client.gets, client.lists, out.String())
	}
}

func TestViewStructuredOutput(t *testing.T) {
	client := &fakeClient{database: dolthub.Database{Owner: "o", Name: "r", Visibility: dolthub.VisibilityPublic}}
	streams, _, out, _ := iostreams.NewTest()
	cmd := NewCmdView(&cmdutil.Factory{}, func(_ context.Context, opts *Options) error {
		opts.IO = streams
		opts.ResolveRepository = fixedRepo
		opts.client = client
		return viewRun(context.Background(), opts)
	})
	cmd.SetArgs([]string{"--json", "owner,name"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "{\"name\":\"r\",\"owner\":\"o\"}\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestViewWebMakesNoAPIRequestAndEscapesSegments(t *testing.T) {
	browser := &fakeBrowser{}
	client := &fakeClient{}
	streams, _, _, _ := iostreams.NewTest()
	opts := &Options{IO: streams, ResolveRepository: func(context.Context, string) (repository.Repository, error) {
		return repository.Repository{Host: "example.test", Owner: "acme/west", Name: "widgets plus"}, nil
	}, Browser: browser, Web: true, client: client}
	if err := viewRun(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if browser.url != "https://example.test/repositories/acme%2Fwest/widgets%20plus" || client.gets != 0 {
		t.Fatalf("URL = %q, API gets = %d", browser.url, client.gets)
	}
}

func TestViewFlagValidation(t *testing.T) {
	for _, args := range [][]string{{"one/repo", "two/repo"}, {"one/repo", "--repo", "two/repo"}, {"--web", "--forks"}, {"--web", "--json", "owner"}} {
		cmd := NewCmdView(&cmdutil.Factory{}, func(context.Context, *Options) error { return nil })
		cmd.SetArgs(args)
		err := cmd.Execute()
		var flagErr *cmdutil.FlagError
		if !errors.As(err, &flagErr) {
			t.Fatalf("args %v: error = %T %v", args, err, err)
		}
	}
}

func fixedRepo(context.Context, string) (repository.Repository, error) {
	return repository.Repository{Host: "example.test", Owner: "o", Name: "r"}, nil
}
