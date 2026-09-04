package create

import (
	"context"
	"strings"
	"testing"

	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

type fakeClient struct {
	request dolthub.CreateDatabaseRequest
	users   int
}

func (f *fakeClient) CurrentUser(context.Context) (dolthub.User, error) {
	f.users++
	return dolthub.User{Username: "alice"}, nil
}
func (f *fakeClient) CreateDatabase(_ context.Context, r dolthub.CreateDatabaseRequest) (dolthub.Database, error) {
	f.request = r
	return dolthub.Database{Owner: r.Owner, Name: r.Name, Description: value(r.Description), Visibility: r.Visibility}, nil
}
func value(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func TestCreateDatabase(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{}
	o := &Options{IO: io, Config: func() (config.Config, error) { return config.NewMemory(), nil }, Name: "org/widgets", Description: "desc", Private: true, client: c}
	if err := createRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if c.users != 0 || c.request.Owner != "org" || c.request.Visibility != dolthub.VisibilityPrivate || !strings.Contains(out.String(), "org/widgets") {
		t.Fatalf("request=%#v users=%d output=%q", c.request, c.users, out.String())
	}
}

func TestCreateDatabasePrintsCustomHost(t *testing.T) {
	io, _, out, _ := iostreams.NewTest()
	c := &fakeClient{}
	cfg := config.NewMemory()
	cfg.DefaultHost = "staging.example.test"
	o := &Options{IO: io, Config: func() (config.Config, error) { return cfg, nil }, Name: "org/widgets", Public: true, client: c}
	if err := createRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "staging.example.test/org/widgets\t") {
		t.Fatal(out.String())
	}
}

func TestCreateUsesCurrentUser(t *testing.T) {
	io, _, _, _ := iostreams.NewTest()
	c := &fakeClient{}
	o := &Options{IO: io, Config: func() (config.Config, error) { return config.NewMemory(), nil }, Name: "widgets", Public: true, client: c}
	if err := createRun(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if c.users != 1 || c.request.Owner != "alice" {
		t.Fatalf("request=%#v users=%d", c.request, c.users)
	}
}

func TestCreateValidation(t *testing.T) {
	io, _, _, _ := iostreams.NewTest()
	f := &cmdutil.Factory{IO: io}
	for _, args := range [][]string{{"name"}, {"name", "--public", "--private"}, {"a", "b", "--public"}} {
		cmd := NewCmdCreate(f, func(context.Context, *Options) error { return nil })
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("args %v expected error", args)
		}
	}
}

func TestParseName(t *testing.T) {
	for _, bad := range []string{"", "/name", "a/b/c"} {
		if _, _, err := parseName(bad); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
}
