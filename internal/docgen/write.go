package docgen

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
)

var pageName = regexp.MustCompile(`^commands/(?:README|[a-z][a-z0-9-]*)\.md$`)
var hashValue = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Write validates ownership before staging output. The manifest is replaced
// last; interruption can leave an inconsistent bundle, detected by --check.
func Write(directory string, bundle Bundle, check bool) error {
	if err := validateBundle(bundle); err != nil {
		return fmt.Errorf("generated manifest: %w", err)
	}
	stat, err := os.Lstat(directory)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if check {
			return fmt.Errorf("missing documentation directory: %s", directory)
		}
		if err = os.MkdirAll(directory, 0755); err != nil {
			return err
		}
	case err != nil:
		return err
	case !stat.IsDir() || stat.Mode()&os.ModeSymlink != 0:
		return fmt.Errorf("output must be a directory, not a symlink: %s", directory)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	existing, err := files(root)
	if err != nil {
		return err
	}
	previous := map[string]bool{}
	if len(existing) > 0 {
		data, err := root.ReadFile("manifest.json")
		if err != nil {
			return fmt.Errorf("nonempty output requires a valid manifest: %w", err)
		}
		previous, err = owned(data)
		if err != nil {
			return err
		}
	}
	var unexpected []string
	for _, name := range existing {
		if !previous[name] {
			unexpected = append(unexpected, name)
		}
	}
	if len(unexpected) > 0 {
		return fmt.Errorf("unexpected files: %s", strings.Join(unexpected, ", "))
	}
	if check {
		var differences []string
		for name, want := range bundle {
			got, err := root.ReadFile(name)
			switch {
			case errors.Is(err, fs.ErrNotExist):
				differences = append(differences, "missing: "+name)
			case err != nil:
				return err
			case !bytes.Equal(got, want):
				differences = append(differences, "changed: "+name)
			}
		}
		for _, name := range existing {
			if _, ok := bundle[name]; !ok {
				differences = append(differences, "unexpected: "+name)
			}
		}
		sort.Strings(differences)
		if len(differences) > 0 {
			return fmt.Errorf("documentation differs:\n%s", strings.Join(differences, "\n"))
		}
		return nil
	}
	// Stage every file before replacing any owned destination. Root confines all
	// filesystem operations, including when another process changes a symlink.
	stage := ".dh-docs-stage-" + rand.Text()
	if err := root.Mkdir(stage, 0700); err != nil {
		return err
	}
	defer func() { _ = root.RemoveAll(stage) }()
	if err := root.Mkdir(path.Join(stage, "commands"), 0755); err != nil {
		return err
	}
	names := make([]string, 0, len(bundle))
	for name, data := range bundle {
		if name != "manifest.json" && !pageName.MatchString(name) {
			return fmt.Errorf("invalid bundle filename %q", name)
		}
		if err := root.WriteFile(path.Join(stage, name), data, 0644); err != nil {
			return err
		}
		if name != "manifest.json" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if err := root.Mkdir("commands", 0755); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	for _, name := range names {
		if err := root.Rename(path.Join(stage, name), name); err != nil {
			return err
		}
	}
	for name := range previous {
		if _, ok := bundle[name]; !ok {
			if err := root.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
	}
	return root.Rename(path.Join(stage, "manifest.json"), "manifest.json")
}

func owned(data []byte) (map[string]bool, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("invalid documentation manifest: %w", err)
	}
	if manifest.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("unsupported manifest schema %d", manifest.SchemaVersion)
	}
	if len(manifest.Pages) == 0 {
		return nil, fmt.Errorf("manifest has no pages")
	}
	names := map[string]bool{"manifest.json": true}
	routes := map[string]bool{}
	for _, page := range manifest.Pages {
		route := BaseRoute + "/" + strings.TrimSuffix(strings.TrimPrefix(page.Filename, "commands/"), ".md")
		if page.Filename == "commands/README.md" {
			route = BaseRoute
		}
		if !pageName.MatchString(page.Filename) || names[page.Filename] || !hashValue.MatchString(page.SHA256) || page.Route != route || routes[page.Route] {
			return nil, fmt.Errorf("invalid or duplicate manifest page %q", page.Filename)
		}
		if _, err := hex.DecodeString(page.SHA256); err != nil {
			return nil, err
		}
		names[page.Filename] = true
		routes[page.Route] = true
	}
	if !names["commands/README.md"] {
		return nil, fmt.Errorf("manifest is missing the command index")
	}
	return names, nil
}
func files(root *os.Root) ([]string, error) {
	var result []string
	err := fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in output: %s", name)
		}
		if entry.IsDir() {
			if name != "commands" {
				return fmt.Errorf("unexpected directory: %s", name)
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular output file: %s", name)
		}
		result = append(result, name)
		return nil
	})
	sort.Strings(result)
	return result, err
}

// Validate the entire new file set before touching a destination. The manifest
// is the ownership boundary, not permission to write other bundle paths.
func validateBundle(bundle Bundle) error {
	names, err := owned(bundle["manifest.json"])
	if err != nil {
		return err
	}
	if len(names) != len(bundle) {
		return fmt.Errorf("bundle file set does not match manifest")
	}
	var manifest Manifest
	if err := json.Unmarshal(bundle["manifest.json"], &manifest); err != nil {
		return err
	}
	for _, page := range manifest.Pages {
		data, ok := bundle[page.Filename]
		if !ok {
			return fmt.Errorf("missing generated page %s", page.Filename)
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != page.SHA256 {
			return fmt.Errorf("hash mismatch for %s", page.Filename)
		}
	}
	for name := range bundle {
		if !names[name] {
			return fmt.Errorf("unowned generated path %q", name)
		}
	}
	anchors := map[string]bool{}
	for _, command := range manifest.Commands {
		if !pageName.MatchString(command.Page) || !names[command.Page] || !commandName.MatchString(command.Anchor) || anchors[command.Anchor] {
			return fmt.Errorf("invalid command target %q", command.Path)
		}
		if !bytes.Contains(bundle[command.Page], []byte("{#"+command.Anchor+"}")) {
			return fmt.Errorf("missing command anchor %s", command.Anchor)
		}
		anchors[command.Anchor] = true
	}
	return nil
}
