package set

import (
	"strings"
	"testing"

	internalconfig "github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/pkg/cmdutil"
)

func TestConstructorAndRun(t *testing.T) {
	m := internalconfig.NewMemory()
	f := &cmdutil.Factory{Config: func() (internalconfig.Config, error) { return m, nil }}
	var got *Options
	cmd := NewCmdSet(f, func(o *Options) error { got = o; return nil })
	cmd.SetArgs([]string{"host", "example.com"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Key != "host" || got.Value != "example.com" {
		t.Fatalf("options = %#v", got)
	}
	if err := setRun(got); err != nil {
		t.Fatal(err)
	}
	if m.Host() != "example.com" || m.Writes != 1 {
		t.Fatalf("config = %#v", m)
	}
}

func TestSetDatabaseAliases(t *testing.T) {
	for _, key := range []string{"db", "repo"} {
		t.Run(key, func(t *testing.T) {
			m := internalconfig.NewMemory()
			opts := &Options{
				Config: func() (internalconfig.Config, error) { return m, nil },
				Key:    key,
				Value:  "owner/database",
			}
			if err := setRun(opts); err != nil {
				t.Fatal(err)
			}
			want := repository.Repository{Host: internalconfig.DefaultHost, Owner: "owner", Name: "database"}
			if got, ok := m.DefaultRepository(); !ok || got != want {
				t.Fatalf("database = %#v, %v; want %#v", got, ok, want)
			}
			if m.Writes != 1 {
				t.Fatalf("writes = %d", m.Writes)
			}
		})
	}
}

func TestDocsUseCanonicalDatabaseKey(t *testing.T) {
	m := internalconfig.NewMemory()
	cmd := NewCmdSet(&cmdutil.Factory{Config: func() (internalconfig.Config, error) { return m, nil }}, nil)
	if !strings.Contains(cmd.Example, "config set db") || strings.Contains(cmd.Example, "config set repo") {
		t.Fatalf("example = %q", cmd.Example)
	}
	docs, err := cmdutil.CommandDocs(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(docs.Arguments[0].Description, "repo remains accepted as an alias") {
		t.Fatalf("key documentation = %q", docs.Arguments[0].Description)
	}
}
