package create

import (
	"context"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/internal/tableprinter"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

var jsonFields = []string{"commit_sha", "created_at", "description", "tag", "title", "updated_at"}

type client interface {
	CreateRelease(context.Context, string, string, dolthub.CreateReleaseRequest) (dolthub.Release, error)
}
type Options struct {
	IO                                               *iostreams.IOStreams
	ResolveRepository                                func(context.Context, string) (repository.Repository, error)
	APIClientForHost                                 func(string) (*dolthub.Client, error)
	Repository, Tag, Title, Target, Notes, NotesFile string
	CreateTag                                        bool
	Exporter                                         cmdutil.Exporter
	client                                           client
}

func NewCmdCreate(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, ResolveRepository: f.ResolveRepository, APIClientForHost: f.APIClientForHost}
	if runF == nil {
		runF = createRun
	}
	c := &cobra.Command{Use: "create TAG", Short: "Create a database release", Args: func(c *cobra.Command, args []string) error {
		if err := cmdutil.ExactArgs(1)(c, args); err != nil {
			return err
		}
		o.Tag = args[0]
		return nil
	}, RunE: func(c *cobra.Command, _ []string) error { return runF(c.Context(), o) }}
	c.PreRunE = func(c *cobra.Command, _ []string) error {
		if o.Title == "" {
			return cmdutil.FlagErrorf("--title is required")
		}
		if o.Target == "" {
			return cmdutil.FlagErrorf("--target is required")
		}
		if c.Flags().Changed("notes") && c.Flags().Changed("notes-file") {
			return cmdutil.FlagErrorf("--notes and --notes-file are mutually exclusive")
		}
		return nil
	}
	cmdutil.AddDatabaseFlag(c, &o.Repository)
	c.Flags().StringVarP(&o.Title, "title", "t", "", "Release title")
	c.Flags().StringVar(&o.Target, "target", "", "Exact target commit SHA")
	c.Flags().StringVarP(&o.Notes, "notes", "n", "", "Release notes")
	c.Flags().StringVarP(&o.NotesFile, "notes-file", "F", "", "Read release notes from file")
	c.Flags().BoolVar(&o.CreateTag, "create-tag", false, "Create the tag if it does not exist")
	cmdutil.AddJSONFlags(c, &o.Exporter, jsonFields)
	return cmdutil.WithDocs(c, "dh release create v1 --db OWNER/people --title \"First dataset\" --target COMMIT_SHA --create-tag", cmdutil.DocMetadata{
		Arguments:   []cmdutil.DocArgument{{Name: "TAG", Description: "Release tag name.", Optional: false}},
		Constraints: []string{"--title and --target are required. --target is an exact commit SHA. Use --create-tag if the tag does not yet exist.", "--notes and --notes-file are mutually exclusive; --notes-file - reads stdin."},
		Output:      "Prints the release tag, title, and commit, or selected JSON fields.",
	})
}
func createRun(ctx context.Context, o *Options) error {
	notes, err := cmdutil.ReadTextSource(o.Notes, o.NotesFile)
	if err != nil {
		return err
	}
	r, err := o.ResolveRepository(ctx, o.Repository)
	if err != nil {
		return err
	}
	c := o.client
	if c == nil {
		c, err = o.APIClientForHost(r.Host)
		if err != nil {
			return err
		}
	}
	var description *string
	if notes != "" {
		description = &notes
	}
	release, err := c.CreateRelease(ctx, r.Owner, r.Name, dolthub.CreateReleaseRequest{Tag: o.Tag, Title: o.Title, CommitSHA: o.Target, Description: description, CreateTagIfNotExists: o.CreateTag})
	if err != nil {
		return err
	}
	if o.Exporter != nil {
		return o.Exporter.Write(o.IO, release)
	}
	t := tableprinter.New(o.IO, "TAG", "TITLE", "COMMIT")
	_ = t.AddRow(release.Tag, release.Title, release.CommitSHA)
	return t.Render()
}
