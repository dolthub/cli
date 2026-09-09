package docgen

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dolthub/cli/internal/buildinfo"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func fixture() *cobra.Command {
	root := &cobra.Command{Use: "dh", Short: "Work with databases"}
	root.PersistentFlags().String("host", "example.test", "Host name")
	group := &cobra.Command{Use: "pr", Short: "Review changes"}
	root.AddCommand(group)
	cmd := &cobra.Command{Use: "create NAME", Short: "Create a review", Long: "Create a review for café data.", Aliases: []string{"new", "add"}, RunE: func(*cobra.Command, []string) error { panic("must not execute") }}
	cmd.PreRunE = func(*cobra.Command, []string) error { panic("must not validate runtime options") }
	cmd.Flags().StringP("body", "b", "a|b\n`quoted`", "Text with |, `code`, <html>, & and [links]")
	cmd.Flags().String("host", "local.test", "Overrides inherited host")
	cmd.Flags().Bool("legacy", false, "Old flag")
	_ = cmd.Flags().MarkDeprecated("legacy", "use body")
	// MarkDeprecated hides a flag in pflag; explicitly visible deprecation fixture.
	cmd.Flags().Lookup("legacy").Hidden = false
	cmd.Flags().Bool("secret", false, "Never export this")
	_ = cmd.Flags().MarkHidden("secret")
	cmdutil.WithDocs(cmd, "dh pr create café --body '```example```'", cmdutil.DocMetadata{Arguments: []cmdutil.DocArgument{{Name: "NAME", Description: "Name to create"}}, Output: "Prints the review.", Modes: []cmdutil.DocMode{{Name: "Structured", Description: "Selected fields.", JSONFields: []string{"id"}}}})
	var exporter cmdutil.Exporter
	cmdutil.AddJSONFlags(cmd, &exporter, []string{"title", "id"})
	group.AddCommand(cmd)
	hidden := &cobra.Command{Use: "internal", Short: "Hidden", Hidden: true}
	hidden.AddCommand(&cobra.Command{Use: "child"})
	root.AddCommand(hidden)
	old := &cobra.Command{Use: "old", Short: "Older group", Deprecated: "use pr"}
	root.AddCommand(old)
	return root
}
func info() buildinfo.Info { return buildinfo.New("1.2.3", strings.Repeat("a", 40), "false", true) }
func generated(t *testing.T, root *cobra.Command) Bundle {
	t.Helper()
	b, err := Generate(root, info())
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestGoldenAndDeterminism(t *testing.T) {
	first := generated(t, fixture())
	second := generated(t, fixture())
	if !reflect.DeepEqual(first, second) {
		t.Fatal("generation is not deterministic")
	}
	for _, name := range []string{"commands/pr.md", "commands/README.md", "manifest.json"} {
		golden := filepath.Join("testdata", strings.ReplaceAll(name, "/", "_"))
		if os.Getenv("UPDATE_DOCGEN_GOLDEN") == "1" {
			if err := os.MkdirAll("testdata", 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(golden, first[name], 0644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first[name], want) {
			t.Fatalf("%s differs from golden; inspect and run UPDATE_DOCGEN_GOLDEN=1 go test ./internal/docgen", name)
		}
	}
	page := string(first["commands/pr.md"])
	for _, text := range []string{"#dh-pr-create", "{#dh-pr-create}", "local.test", "&#124;", "&#96;", "&lt;html&gt;", "````bash"} {
		if !strings.Contains(page, text) {
			t.Errorf("missing %q", text)
		}
	}
	start := strings.Index(page, "## dh pr create")
	if start < 0 {
		t.Fatal("missing create section")
	}
	page = page[start:]
	for _, text := range []string{"example.test", "--secret"} {
		if strings.Contains(page, text) {
			t.Errorf("unexpected %q", text)
		}
	}
	if !strings.Contains(string(first["commands/old.md"]), "Deprecated: use pr") {
		t.Fatal("lost visible deprecated command")
	}
}
func TestGroupingAndDefaults(t *testing.T) {
	root := fixture()
	base := generated(t, root)
	group, _, _ := root.Find([]string{"pr"})
	leaf := cmdutil.WithDocs(&cobra.Command{Use: "list", Short: "List reviews", Run: func(*cobra.Command, []string) {}}, "dh pr list", cmdutil.DocMetadata{Output: "Prints reviews."})
	group.AddCommand(leaf)
	withLeaf := generated(t, root)
	if len(base) != len(withLeaf) {
		t.Fatal("nested command created a page")
	}
	if !strings.Contains(string(withLeaf["commands/pr.md"]), "{#dh-pr-list}") {
		t.Fatal("missing nested command")
	}
	root.AddCommand(&cobra.Command{Use: "db", Short: "Databases"})
	if len(generated(t, root)) != len(base)+1 {
		t.Fatal("top-level command must add one page")
	}
	cmd, _, _ := root.Find([]string{"pr", "create"})
	if err := cmd.Flags().Set("body", "invocation value"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(generated(t, root)["commands/pr.md"]), "invocation value") {
		t.Fatal("exported invocation value instead of default")
	}
}
func TestInvalidMetadata(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*cobra.Command)
	}{
		{"example", func(c *cobra.Command) { c.Example = "" }},
		{"metadata", func(c *cobra.Command) { delete(c.Annotations, "docs:metadata") }},
		{"malformed", func(c *cobra.Command) { c.Annotations["docs:metadata"] = "{" }},
		{"output", func(c *cobra.Command) { cmdutil.WithDocs(c, "example", cmdutil.DocMetadata{}) }},
		{"arguments", func(c *cobra.Command) { cmdutil.WithDocs(c, "example", cmdutil.DocMetadata{Output: "output"}) }},
		{"unknown mode field", func(c *cobra.Command) {
			m, _ := cmdutil.CommandDocs(c)
			m.Modes[0].JSONFields = []string{"unknown"}
			cmdutil.WithDocs(c, c.Example, m)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := fixture()
			c, _, _ := root.Find([]string{"pr", "create"})
			tt.change(c)
			if _, err := Generate(root, info()); err == nil {
				t.Fatal("accepted incomplete docs")
			}
		})
	}
	root := fixture()
	root.AddCommand(&cobra.Command{Use: "pr-create", Short: "Collision"})
	if _, err := Generate(root, info()); err == nil {
		t.Fatal("accepted duplicate anchor")
	}
}
func TestCommandInventory(t *testing.T) {
	bundle := generated(t, fixture())
	var manifest Manifest
	if err := json.Unmarshal(bundle["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	for _, cmd := range manifest.Commands {
		page, ok := bundle[cmd.Page]
		if !ok {
			t.Fatalf("missing page %s", cmd.Page)
		}
		if !strings.Contains(string(page), "{#"+cmd.Anchor+"}") {
			t.Errorf("missing anchor %s", cmd.Anchor)
		}
		if strings.Contains(cmd.Path, "internal") {
			t.Fatal("hidden command was exported")
		}
	}
}
