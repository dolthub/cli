package create

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

var jsonFields = []string{"description", "fork_network_count", "last_write_at", "name", "network_root", "owner", "parent", "size_bytes", "star_count", "visibility"}

type client interface {
	CurrentUser(context.Context) (dolthub.User, error)
	CreateDatabase(context.Context, dolthub.CreateDatabaseRequest) (dolthub.Database, error)
}

type Options struct {
	IO                *iostreams.IOStreams
	Config            func() (config.Config, error)
	APIClientForHost  func(string) (*dolthub.Client, error)
	Name, Description string
	Public, Private   bool
	Exporter          cmdutil.Exporter
	client            client
}

func NewCmdCreate(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, Config: f.Config, APIClientForHost: f.APIClientForHost}
	if runF == nil {
		runF = createRun
	}
	c := &cobra.Command{Use: "create [OWNER/]NAME", Short: "Create a database repository", Args: func(c *cobra.Command, args []string) error {
		if len(args) > 1 {
			return cmdutil.FlagErrorf("%s accepts at most one argument", c.CommandPath())
		}
		if len(args) == 1 {
			o.Name = args[0]
		}
		return nil
	}, RunE: func(c *cobra.Command, _ []string) error { return runF(c.Context(), o) }}
	c.PreRunE = func(*cobra.Command, []string) error {
		if o.Public && o.Private {
			return cmdutil.FlagErrorf("--public and --private are mutually exclusive")
		}
		if !o.IO.IsStdinTTY() && strings.TrimSpace(o.Name) == "" {
			return cmdutil.FlagErrorf("database name is required in non-interactive use")
		}
		if !o.IO.IsStdinTTY() && !o.Public && !o.Private {
			return cmdutil.FlagErrorf("--public or --private is required in non-interactive use")
		}
		return nil
	}
	c.Flags().StringVarP(&o.Description, "description", "d", "", "Description of the database")
	c.Flags().BoolVar(&o.Public, "public", false, "Make the database public")
	c.Flags().BoolVar(&o.Private, "private", false, "Make the database private")
	cmdutil.AddJSONFlags(c, &o.Exporter, jsonFields)
	return cmdutil.WithDocs(c, "dh db create OWNER/people --private", cmdutil.DocMetadata{
		Arguments:   []cmdutil.DocArgument{{Name: "[OWNER/]NAME", Description: "Database name; owner defaults to the authenticated user. Interactive use can prompt for the name.", Optional: true}},
		Constraints: []string{"--public and --private are mutually exclusive. Noninteractive use requires a name and one visibility flag. Interactive use prompts for missing name, visibility, and optionally description."},
		Output:      "Prints the created database identifier and browser URL, or the selected database fields with --json.",
	})
}

func createRun(ctx context.Context, o *Options) error {
	name := strings.TrimSpace(o.Name)
	var err error
	if name == "" {
		name, err = cmdutil.PromptLine(o.IO, "Database name")
		if err != nil {
			return err
		}
	}
	owner, database, err := parseName(name)
	if err != nil {
		return err
	}
	visibility := dolthub.VisibilityPublic
	if !o.Public && !o.Private {
		choice, e := cmdutil.PromptLine(o.IO, "Visibility (public/private)")
		if e != nil {
			return e
		}
		switch strings.ToLower(choice) {
		case "public":
			o.Public = true
		case "private":
			o.Private = true
		default:
			return cmdutil.FlagErrorf("visibility must be public or private")
		}
	}
	if o.Private {
		visibility = dolthub.VisibilityPrivate
	}
	cfg, err := o.Config()
	if err != nil {
		return err
	}
	c := o.client
	if c == nil {
		c, err = o.APIClientForHost(cfg.Host())
		if err != nil {
			return err
		}
	}
	if owner == "" {
		user, e := c.CurrentUser(ctx)
		if e != nil {
			return e
		}
		owner = user.Username
	}
	description := strings.TrimSpace(o.Description)
	if description == "" && o.IO.IsStdinTTY() {
		description, err = cmdutil.PromptLine(o.IO, "Description (optional)")
		if err != nil {
			return err
		}
	}
	var descriptionPtr *string
	if description != "" {
		descriptionPtr = &description
	}
	db, err := c.CreateDatabase(ctx, dolthub.CreateDatabaseRequest{Owner: owner, Name: database, Description: descriptionPtr, Visibility: visibility})
	if err != nil {
		return err
	}
	if o.Exporter != nil {
		return o.Exporter.Write(o.IO, db)
	}
	identifier := db.Owner + "/" + db.Name
	if cfg.Host() != config.DefaultHost {
		identifier = cfg.Host() + "/" + identifier
	}
	_, err = fmt.Fprintf(o.IO.Out, "%s\t%s\n", identifier, databaseURL(cfg.Host(), db.Owner, db.Name))
	return err
}

func parseName(value string) (string, string, error) {
	parts := strings.Split(value, "/")
	if len(parts) > 2 || strings.TrimSpace(parts[len(parts)-1]) == "" {
		return "", "", cmdutil.FlagErrorf("database must be NAME or OWNER/NAME")
	}
	if len(parts) == 1 {
		return "", parts[0], nil
	}
	if strings.TrimSpace(parts[0]) == "" {
		return "", "", cmdutil.FlagErrorf("database owner must not be empty")
	}
	return parts[0], parts[1], nil
}

func databaseURL(host, owner, name string) string {
	path := "/repositories/" + owner + "/" + name
	raw := "/repositories/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
	return (&url.URL{Scheme: "https", Host: host, Path: path, RawPath: raw}).String()
}
