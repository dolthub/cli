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
	pages map[string]struct {
		items []dolthub.Release
		next  string
	}
	calls []string
}

func (f *fakeClient) ListReleases(_ context.Context, _, _, token string) ([]dolthub.Release, string, error) {
	f.calls = append(f.calls, token)
	p := f.pages[token]
	return p.items, p.next, nil
}

type fakeBrowser struct {
	url string
	err error
}

func (f *fakeBrowser) Browse(u string) error { f.url = u; return f.err }
func resolver(context.Context, string) (repository.Repository, error) {
	return repository.Repository{Host: "example.test", Owner: "acme/west", Name: "widgets"}, nil
}

func TestViewFindsExactReleaseAndStopsEarly(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	c := &fakeClient{pages: map[string]struct {
		items []dolthub.Release
		next  string
	}{"": {[]dolthub.Release{{Tag: "v1", Title: "One", CreatedAt: now, UpdatedAt: now}}, "next"}, "next": {[]dolthub.Release{{Tag: "v2", Title: "Two", CommitSHA: "abc", CreatedAt: now, UpdatedAt: now}}, "unused"}}}
	o := &Options{IO: io, ResolveRepository: resolver, Tag: "v2", client: c}
	if err := viewRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if len(c.calls) != 2 || !strings.Contains(out.String(), "Two") {
		t.Fatalf("calls=%v output=%q", c.calls, out.String())
	}
}

func TestViewNotFoundAndRepeatedToken(t *testing.T) {
	io, _, _, _ := iostreams.NewTest()
	for name, c := range map[string]*fakeClient{"not found": {pages: map[string]struct {
		items []dolthub.Release
		next  string
	}{"": {nil, ""}}}, "repeat": {pages: map[string]struct {
		items []dolthub.Release
		next  string
	}{"": {nil, "same"}, "same": {nil, "same"}}}} {
		t.Run(name, func(t *testing.T) {
			err := viewRun(context.Background(), &Options{IO: io, ResolveRepository: resolver, Tag: "missing", client: c})
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestViewWebSkipsAPI(t *testing.T) {
	b := &fakeBrowser{}
	want := errors.New("browser")
	b.err = want
	err := viewRun(context.Background(), &Options{ResolveRepository: resolver, Browser: b, Tag: "v1/a", Web: true})
	if !errors.Is(err, want) || b.url != "https://example.test/repositories/acme%2Fwest/widgets/releases" {
		t.Fatalf("url=%q err=%v", b.url, err)
	}
}

func TestViewCommandValidationAndJSONFields(t *testing.T) {
	io, _, _, _ := iostreams.NewTest()
	f := &cmdutil.Factory{IO: io}
	c := NewCmdView(f, func(context.Context, *Options) error { return nil })
	c.SetArgs([]string{"v1", "--web", "--json", "tag"})
	if err := c.Execute(); err == nil {
		t.Fatal("expected conflicting flag error")
	}
	if !strings.Contains(c.Annotations["help:json-fields"], "commit_sha") {
		t.Fatal(c.Annotations)
	}
}
