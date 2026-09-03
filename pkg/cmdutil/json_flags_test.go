package cmdutil

import (
	"strings"
	"testing"

	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

func TestAddJSONFlagsValidatesAndExports(t *testing.T) {
	cmd := &cobra.Command{Use: "test", RunE: func(*cobra.Command, []string) error { return nil }}
	var exporter Exporter
	AddJSONFlags(cmd, &exporter, []string{"name", "size"})
	cmd.SetArgs([]string{"--json", "name", "--jq", ".[0].name"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	streams, _, out, _ := iostreams.NewTest()
	if err := exporter.Write(streams, []map[string]any{{"name": "widgets", "size": 12, "secret": "no"}}); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "widgets" {
		t.Fatalf("output = %q", got)
	}
}

func TestAddJSONFlagsRejectsInvalidCombinations(t *testing.T) {
	for _, args := range [][]string{{"--jq", ".name"}, {"--template", "{{.name}}"}, {"--json", "unknown"}} {
		cmd := &cobra.Command{Use: "test", RunE: func(*cobra.Command, []string) error { return nil }, SilenceUsage: true, SilenceErrors: true}
		var exporter Exporter
		AddJSONFlags(cmd, &exporter, []string{"name"})
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Errorf("args %v unexpectedly succeeded", args)
		}
	}
}

func TestJSONTemplate(t *testing.T) {
	cmd := &cobra.Command{Use: "test", RunE: func(*cobra.Command, []string) error { return nil }}
	var exporter Exporter
	AddJSONFlags(cmd, &exporter, []string{"name"})
	cmd.SetArgs([]string{"--json", "name", "--template", "{{.name}}"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	streams, _, out, _ := iostreams.NewTest()
	if err := exporter.Write(streams, map[string]any{"name": "widgets"}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "widgets" {
		t.Fatalf("output = %q", out.String())
	}
}
