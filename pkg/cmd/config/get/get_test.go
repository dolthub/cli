package get

import (
	"testing"

	internalconfig "github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

func TestConstructorAndRun(t *testing.T) {
	streams, _, out, _ := iostreams.NewTest()
	m := internalconfig.NewMemory()
	m.SetDefaultRepository(repository.Repository{Host: "www.dolthub.com", Owner: "o", Name: "r"})
	f := &cmdutil.Factory{IO: streams, Config: func() (internalconfig.Config, error) { return m, nil }}
	var got *Options
	cmd := NewCmdGet(f, func(o *Options) error { got = o; return nil })
	cmd.SetArgs([]string{"db"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Key != "db" {
		t.Fatalf("options = %#v", got)
	}
	if err := getRun(got); err != nil {
		t.Fatal(err)
	}
	if out.String() != "o/r\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestGetRepoAlias(t *testing.T) {
	streams, _, out, _ := iostreams.NewTest()
	m := internalconfig.NewMemory()
	m.SetDefaultRepository(repository.Repository{Host: "www.dolthub.com", Owner: "o", Name: "r"})

	for _, key := range []string{"db", "repo"} {
		out.Reset()
		if err := getRun(&Options{IO: streams, Config: func() (internalconfig.Config, error) { return m, nil }, Key: key}); err != nil {
			t.Fatalf("get %s: %v", key, err)
		}
		if out.String() != "o/r\n" {
			t.Fatalf("get %s output = %q", key, out.String())
		}
	}
}
