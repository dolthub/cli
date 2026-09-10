package root

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dolthub/cli/internal/buildinfo"
	"github.com/dolthub/cli/internal/docgen"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestDocumentationCoversRealTree(t *testing.T) {
	streams, _, _, _ := iostreams.NewTest()
	// Nil services make any accidental command execution fail immediately.
	root := NewCmdRoot(&cmdutil.Factory{IO: streams, AppVersion: "dev"})
	bundle, err := docgen.Generate(root, buildinfo.New("dev", "", "", false))
	if err != nil {
		t.Fatal(err)
	}
	var manifest docgen.Manifest
	if err := json.Unmarshal(bundle["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Pages) != 1 {
		t.Fatalf("want one command reference page; got %d", len(manifest.Pages))
	}
	items := map[string]docgen.Command{}
	for _, item := range manifest.Commands {
		items[item.Path] = item
	}
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd.Hidden {
			return
		}
		item, ok := items[cmd.CommandPath()]
		if !ok {
			t.Fatalf("missing command %s", cmd.CommandPath())
		}
		text := string(bundle[item.Page])
		needle := "## " + item.Path + " {#" + item.Anchor + "}"
		start := strings.Index(text, needle)
		if start < 0 {
			t.Fatalf("missing section %s", needle)
		}
		section := text[start:]
		if end := strings.Index(section[3:], "\n## "); end >= 0 {
			section = section[:end+3]
		}
		cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
			if !flag.Hidden && !strings.Contains(section, "<code>--"+flag.Name+"</code>") {
				t.Errorf("%s missing flag %s", cmd.CommandPath(), flag.Name)
			}
		})
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)
	foundDatabaseEnv := false
	for name, data := range bundle {
		if strings.Contains(string(data), "--repo") || strings.Contains(string(data), "dh-generate-docs") {
			t.Errorf("hidden metadata in %s", name)
		}
		if strings.Contains(string(data), "DH_REPO") {
			t.Errorf("legacy database environment alias surfaced in %s", name)
		}
		foundDatabaseEnv = foundDatabaseEnv || strings.Contains(string(data), "DH_DB")
	}
	if !foundDatabaseEnv {
		t.Fatal("canonical DH_DB environment variable is missing from generated documentation")
	}
	if len(bundle) != 2 {
		t.Fatal("unexpected generated file")
	}
}
func TestHiddenGenerateCommand(t *testing.T) {
	streams, _, _, _ := iostreams.NewTest()
	factory := &cmdutil.Factory{IO: streams, AppVersion: "dev"}
	for _, args := range [][]string{{"--help"}, {"completion", "bash"}, {"completion", "fish"}, {"completion", "powershell"}, {"completion", "zsh"}, {"__complete", ""}} {
		root := NewCmdRoot(factory)
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "generate-docs") {
			t.Errorf("hidden command advertised by %v", args)
		}
	}
	root := NewCmdRoot(factory)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"generate-docs", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "--output") {
		t.Fatal("explicit help unavailable")
	}
	dir := filepath.Join(t.TempDir(), "bundle")
	t.Setenv("DH_TOKEN", "sentinel-token-never-export")
	t.Setenv("DH_HOST", "not a hostname")
	t.Setenv("DH_DB", "not a database")
	for _, extra := range [][]string{nil, {"--check"}} {
		root := NewCmdRoot(factory)
		root.SetOut(&out)
		root.SetArgs(append([]string{"generate-docs", "--output", dir}, extra...))
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "sentinel") {
		t.Fatal("exported environment")
	}
}
