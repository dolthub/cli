package docgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestWriteCheckAndOwnedRemoval(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "docs")
	root := fixture()
	root.AddCommand(&cobra.Command{Use: "extra", Short: "Extra group"})
	before := generated(t, root)
	if err := Write(dir, before, true); err == nil {
		t.Fatal("check accepted missing directory")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("check created output")
	}
	for _, check := range []bool{false, false, true} {
		if err := Write(dir, before, check); err != nil {
			t.Fatal(err)
		}
	}
	after := generated(t, fixture())
	if err := Write(dir, after, true); err == nil || !strings.Contains(err.Error(), "unexpected: commands/extra.md") {
		t.Fatalf("check: %v", err)
	}
	if err := Write(dir, after, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "commands/extra.md")); !os.IsNotExist(err) {
		t.Fatal("obsolete generated page remains")
	}
	if err := os.WriteFile(filepath.Join(dir, "commands/pr.md"), []byte("edited"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Write(dir, after, true); err == nil || !strings.Contains(err.Error(), "changed: commands/pr.md") {
		t.Fatalf("check: %v", err)
	}
	if err := Write(dir, after, false); err != nil {
		t.Fatal(err)
	}
	if err := Write(dir, after, true); err != nil {
		t.Fatal(err)
	}
}
func TestRefuseUnownedAndUnsafeOutput(t *testing.T) {
	bundle := generated(t, fixture())
	for _, name := range []string{"unowned", "unexpected", "directory", "bad manifest", "schema", "traversal", "duplicate", "hash", "symlink", "root symlink"} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "docs")
			if err := Write(dir, bundle, false); err != nil {
				t.Fatal(err)
			}
			keep := filepath.Join(dir, "commands/pr.md")
			original, err := os.ReadFile(keep)
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "unowned":
				if err := os.Remove(filepath.Join(dir, "manifest.json")); err != nil {
					t.Fatal(err)
				}
			case "unexpected":
				if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("mine"), 0644); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(filepath.Join(dir, "notes"), 0755); err != nil {
					t.Fatal(err)
				}
			case "bad manifest":
				if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{"), 0644); err != nil {
					t.Fatal(err)
				}
			case "schema", "traversal", "duplicate", "hash":
				var m Manifest
				if err := json.Unmarshal(bundle["manifest.json"], &m); err != nil {
					t.Fatal(err)
				}
				switch name {
				case "schema":
					m.SchemaVersion++
				case "traversal":
					m.Pages[0].Filename = "../outside.md"
				case "duplicate":
					m.Pages = append(m.Pages, m.Pages[0])
				case "hash":
					m.Pages[0].SHA256 = "invalid"
				}
				data, _ := json.Marshal(m)
				if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0644); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(keep, filepath.Join(dir, "link")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			case "root symlink":
				link := dir + "-link"
				if err := os.Symlink(dir, link); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				dir = link
			}
			for _, check := range []bool{true, false} {
				if err := Write(dir, bundle, check); err == nil {
					t.Fatalf("accepted %s (check=%t)", name, check)
				}
			}
			got, err := os.ReadFile(keep)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(original) {
				t.Fatal("modified existing page on validation failure")
			}
		})
	}
}
func TestInvalidNewBundleDoesNotWrite(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "docs")
	bundle := generated(t, fixture())
	bundle["manifest.json"] = []byte("{}")
	if err := Write(dir, bundle, false); err == nil {
		t.Fatal("accepted invalid generation")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("invalid generation created output")
	}
}

func TestNewBundleIntegrity(t *testing.T) {
	for _, kind := range []string{"hash", "unowned path", "missing page", "missing anchor"} {
		t.Run(kind, func(t *testing.T) {
			bundle := generated(t, fixture())
			switch kind {
			case "hash":
				bundle["commands/pr.md"] = []byte("changed")
			case "unowned path":
				bundle["../outside.md"] = []byte("no")
			case "missing page":
				delete(bundle, "commands/pr.md")
			case "missing anchor":
				var m Manifest
				if err := json.Unmarshal(bundle["manifest.json"], &m); err != nil {
					t.Fatal(err)
				}
				m.Commands[0].Anchor = "missing-anchor"
				data, _ := json.Marshal(m)
				bundle["manifest.json"] = data
			}
			dir := filepath.Join(t.TempDir(), "docs")
			if err := Write(dir, bundle, false); err == nil {
				t.Fatal("accepted invalid bundle")
			}
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Fatal("invalid bundle created directory")
			}
		})
	}
}
