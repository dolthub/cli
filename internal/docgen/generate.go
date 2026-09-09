// Package docgen exports a deterministic reference without executing commands.
package docgen

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/dolthub/cli/internal/buildinfo"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const BaseRoute = "/products/dolthub/cli/commands"
const SchemaVersion = 1
const GeneratorVersion = 1

type Command struct {
	Path    string   `json:"path"`
	Summary string   `json:"summary"`
	Page    string   `json:"page"`
	Anchor  string   `json:"anchor"`
	Aliases []string `json:"aliases,omitempty"`
}
type Page struct {
	Filename    string `json:"filename"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Route       string `json:"route"`
	SHA256      string `json:"sha256"`
}
type Manifest struct {
	SchemaVersion    int `json:"schema_version"`
	GeneratorVersion int `json:"generator_version"`
	buildinfo.Info
	Commands []Command `json:"commands"`
	Pages    []Page    `json:"pages"`
}
type Bundle map[string][]byte

type entry struct {
	command    *cobra.Command
	item       Command
	metadata   cmdutil.DocMetadata
	flags      []*pflag.Flag
	jsonFields []string
}

var commandName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Generate uses declared metadata only: no Execute, hooks, flag parsing, or I/O.
func Generate(root *cobra.Command, info buildinfo.Info) (Bundle, error) {
	if root.Name() != "dh" {
		return nil, fmt.Errorf("documentation root must be dh")
	}
	var entries []entry
	anchors := map[string]bool{}
	var walk func(*cobra.Command, string) error
	walk = func(cmd *cobra.Command, page string) error {
		if cmd.Hidden {
			return nil
		}
		if !commandName.MatchString(cmd.Name()) {
			return fmt.Errorf("invalid command name %q", cmd.Name())
		}
		if strings.TrimSpace(cmd.Short) == "" {
			return fmt.Errorf("%s: missing summary", cmd.CommandPath())
		}
		if cmd.Parent() == root {
			page = "commands/" + cmd.Name() + ".md"
		}
		anchor := strings.ReplaceAll(cmd.CommandPath(), " ", "-")
		if anchors[anchor] {
			return fmt.Errorf("duplicate command anchor %q", anchor)
		}
		anchors[anchor] = true
		aliases := append([]string(nil), cmd.Aliases...)
		sort.Strings(aliases)
		e := entry{command: cmd, item: Command{Path: cmd.CommandPath(), Summary: cmd.Short, Page: page, Anchor: anchor, Aliases: aliases}}
		if cmd.Runnable() {
			metadata, err := cmdutil.CommandDocs(cmd)
			if err != nil {
				return err
			}
			if strings.TrimSpace(cmd.Example) == "" || strings.TrimSpace(metadata.Output) == "" {
				return fmt.Errorf("%s: example and output documentation are required", cmd.CommandPath())
			}
			if len(strings.Fields(cmd.Use)) > 1 && len(metadata.Arguments) == 0 {
				return fmt.Errorf("%s: argument documentation is required", cmd.CommandPath())
			}
			for _, arg := range metadata.Arguments {
				if arg.Name == "" || arg.Description == "" {
					return fmt.Errorf("%s: incomplete argument documentation", cmd.CommandPath())
				}
			}
			e.metadata = metadata
		}
		// Normalize help for fresh trees; its default is not an invocation value.
		cmd.InitDefaultHelpFlag()
		flags := map[string]*pflag.Flag{}
		cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) { flags[f.Name] = f })
		cmd.LocalFlags().VisitAll(func(f *pflag.Flag) { flags[f.Name] = f })
		for _, flag := range flags {
			if !flag.Hidden {
				e.flags = append(e.flags, flag)
			}
		}
		sort.Slice(e.flags, func(i, j int) bool { return e.flags[i].Name < e.flags[j].Name })
		if fields := cmd.Annotations["help:json-fields"]; fields != "" {
			e.jsonFields = strings.Split(fields, ",")
			sort.Strings(e.jsonFields)
		}
		for _, mode := range e.metadata.Modes {
			if mode.Name == "" || mode.Description == "" {
				return fmt.Errorf("%s: incomplete mode documentation", cmd.CommandPath())
			}
			for _, field := range mode.JSONFields {
				if !contains(e.jsonFields, field) {
					return fmt.Errorf("%s: unknown mode JSON field %q", cmd.CommandPath(), field)
				}
			}
		}
		entries = append(entries, e)
		children := append([]*cobra.Command(nil), cmd.Commands()...)
		sort.Slice(children, func(i, j int) bool { return children[i].Name() < children[j].Name() })
		for _, child := range children {
			// Cobra's built-in help command is explained in the index.
			if child.Name() == "help" && child.Parent() == root {
				continue
			}
			if err := walk(child, page); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root, "commands/README.md"); err != nil {
		return nil, err
	}
	manifest := Manifest{SchemaVersion: SchemaVersion, GeneratorVersion: GeneratorVersion, Info: info}
	bundle := Bundle{}
	groups := map[string][]entry{}
	for _, e := range entries {
		manifest.Commands = append(manifest.Commands, e.item)
		groups[e.item.Page] = append(groups[e.item.Page], e)
	}
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		group := groups[name]
		title, description, route := group[0].item.Path, group[0].item.Summary, BaseRoute+"/"+group[0].command.Name()
		if name == "commands/README.md" {
			title = "DoltHub CLI command reference"
			description = "Commands, flags, examples, and output formats for dh."
			route = BaseRoute
		}
		var out bytes.Buffer
		fmt.Fprintf(&out, "---\ntitle: %s\ndescription: %s\n---\n\n", strconv.Quote(title), strconv.Quote(description))
		// JSON string encoding avoids closing a comment through unusual dev versions.
		provenance, _ := json.Marshal(info)
		fmt.Fprintf(&out, "<!-- Generated by dh generate-docs (renderer %d). Do not edit.\nProvenance: %s\nRegenerate with: dh generate-docs --output DIRECTORY\n-->\n\n", GeneratorVersion, bytes.ReplaceAll(provenance, []byte("--"), []byte(`\u002d\u002d`)))
		if name == "commands/README.md" {
			fmt.Fprintf(&out, "Reference for dh %s.\n\n", code(info.Version))
			if !info.PublicationReady {
				out.WriteString("Development reference; this bundle is not stamped for publication.\n\n")
			}
			out.WriteString("Use `dh --help`, `dh <command> --help`, or `dh help <command>` to explore commands in the terminal.\n\n")
			renderCommand(&out, group[0], entries)
			out.WriteString("## All commands\n\n| Command | Purpose |\n| --- | --- |\n")
			for _, e := range entries[1:] {
				fmt.Fprintf(&out, "| [%s](%s) | %s |\n", cell(e.item.Path), target(e.item), cell(e.item.Summary))
			}
		} else {
			for _, e := range group {
				renderCommand(&out, e, entries)
			}
		}
		data := append(bytes.TrimRight(out.Bytes(), "\n"), '\n')
		hash := sha256.Sum256(data)
		bundle[name] = data
		manifest.Pages = append(manifest.Pages, Page{Filename: name, Title: title, Description: description, Route: route, SHA256: hex.EncodeToString(hash[:])})
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	bundle["manifest.json"] = append(data, '\n')
	return bundle, nil
}

func renderCommand(out *bytes.Buffer, e entry, entries []entry) {
	cmd := e.command
	fmt.Fprintf(out, "## %s {#%s}\n\n", cmd.CommandPath(), e.item.Anchor)
	description := cmd.Long
	if description == "" {
		description = cmd.Short
	}
	fmt.Fprintf(out, "%s\n\n", description)
	if cmd.Deprecated != "" {
		fmt.Fprintf(out, "Deprecated: %s\n\n", cmd.Deprecated)
	}
	if len(e.item.Aliases) > 0 {
		fmt.Fprintf(out, "Aliases: %s.\n\n", strings.Join(e.item.Aliases, ", "))
	}
	out.WriteString("### Usage\n\n")
	usage := cmd.UseLine()
	if !cmd.Runnable() && cmd.HasAvailableSubCommands() {
		usage = strings.Replace(usage, " [flags]", " [command] [flags]", 1)
	}
	fence(out, "text", usage)
	if len(e.metadata.Arguments) > 0 {
		out.WriteString("### Arguments\n\n| Argument | Optional | Repeated | Description |\n| --- | --- | --- | --- |\n")
		for _, a := range e.metadata.Arguments {
			fmt.Fprintf(out, "| %s | %t | %t | %s |\n", code(a.Name), a.Optional, a.Repeated, cell(a.Description))
		}
		out.WriteByte('\n')
	}
	if len(e.flags) > 0 {
		out.WriteString("### Flags\n\n| Flag | Short | Type | Default | Description |\n| --- | --- | --- | --- | --- |\n")
		for _, f := range e.flags {
			shorthand := ""
			if f.Shorthand != "" && f.ShorthandDeprecated == "" {
				shorthand = "-" + f.Shorthand
			}
			usage := f.Usage
			if f.Deprecated != "" {
				usage += " Deprecated: " + f.Deprecated
			}
			value := f.DefValue
			if f.Value.Type() == "string" {
				value = strconv.Quote(value)
			}
			fmt.Fprintf(out, "| %s | %s | %s | %s | %s |\n", code("--"+f.Name), code(shorthand), cell(f.Value.Type()), code(value), cell(usage))
		}
		out.WriteByte('\n')
	}
	if len(e.metadata.Constraints) > 0 {
		out.WriteString("### Constraints\n\n")
		for _, s := range e.metadata.Constraints {
			fmt.Fprintf(out, "- %s\n", s)
		}
		out.WriteByte('\n')
	}
	for _, mode := range e.metadata.Modes {
		fmt.Fprintf(out, "### %s\n\n%s\n\n", mode.Name, mode.Description)
		if len(mode.JSONFields) > 0 {
			fmt.Fprintf(out, "JSON fields: %s.\n\n", fieldList(mode.JSONFields))
		}
	}
	if len(e.jsonFields) > 0 {
		fmt.Fprintf(out, "### JSON fields\n\n%s\n\n", fieldList(e.jsonFields))
	}
	if e.metadata.Output != "" {
		fmt.Fprintf(out, "### Output\n\n%s\n\n", e.metadata.Output)
	}
	if cmd.Example != "" {
		out.WriteString("### Examples\n\n")
		fence(out, "bash", cmd.Example)
	}
	var related []Command
	for _, other := range entries {
		if other.command == cmd.Parent() || other.command.Parent() == cmd {
			related = append(related, other.item)
		}
	}
	if len(related) > 0 {
		out.WriteString("### Related commands\n\n")
		for _, item := range related {
			fmt.Fprintf(out, "- [%s](%s)\n", item.Path, target(item))
		}
		out.WriteByte('\n')
	}
}
func target(c Command) string {
	page := strings.TrimSuffix(strings.TrimPrefix(c.Page, "commands/"), ".md")
	if page == "README" {
		return BaseRoute + "#" + c.Anchor
	}
	return BaseRoute + "/" + page + "#" + c.Anchor
}
func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func cell(value string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "|", "&#124;", "`", "&#96;", "\r\n", "<br>", "\n", "<br>", "\r", "<br>", "[", "&#91;", "]", "&#93;", "*", "&#42;", "_", "&#95;", "\\", "&#92;")
	return r.Replace(value)
}
func code(value string) string {
	if value == "" {
		return ""
	}
	return "<code>" + cell(value) + "</code>"
}
func fieldList(fields []string) string {
	fields = append([]string(nil), fields...)
	sort.Strings(fields)
	for i := range fields {
		fields[i] = code(fields[i])
	}
	return strings.Join(fields, ", ")
}
func fence(out *bytes.Buffer, language, value string) {
	ticks := "```"
	for strings.Contains(value, ticks) {
		ticks += "`"
	}
	fmt.Fprintf(out, "%s%s\n%s\n%s\n\n", ticks, language, strings.TrimSpace(value), ticks)
}
