package set

import (
	"testing"

	internalconfig "github.com/dolthub/cli/internal/config"
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
