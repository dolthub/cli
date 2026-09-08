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
	if version, _, err := cmd.Find([]string{"version"}); err != nil || version.Name() != "version" {
		t.Fatalf("version command not found: %v", err)
	}
	if completion, _, err := cmd.Find([]string{"completion"}); err != nil || completion.Name() != "completion" {
		t.Fatalf("completion command not found: %v", err)
	}
	if sql, _, err := cmd.Find([]string{"sql"}); err != nil || sql.Name() != "sql" {
		t.Fatalf("sql command not found: %v", err)
	}
	if tableImport, _, err := cmd.Find([]string{"table", "import"}); err != nil || tableImport.Name() != "import" {
		t.Fatalf("table import command not found: %v", err)
	}
}
