package root

import (
	"testing"

	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

func TestNewCmdRootDoesNotRequireExternalState(t *testing.T) {
	streams, _, _, _ := iostreams.NewTest()
	cmd := NewCmdRoot(&cmdutil.Factory{AppVersion: "test", IO: streams})

	if cmd.Use != "dh" {
		t.Fatalf("Use = %q, want %q", cmd.Use, "dh")
	}
	if cmd.Commands()[0].Name() != "version" {
		t.Fatalf("first command = %q, want %q", cmd.Commands()[0].Name(), "version")
	}
}
