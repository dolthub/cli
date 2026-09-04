package view

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/dolthub/cli/internal/browser"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/internal/tableprinter"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

var jsonFields = []string{"comments", "created_at", "creator", "description", "from_branch", "pull_number", "state", "title", "to_branch"}

type client interface {
	GetPull(context.Context, string, string, int64) (dolthub.Pull, error)
	ListPullComments(context.Context, string, string, int64) ([]dolthub.PullComment, error)
}

type Options struct {
	IO                *iostreams.IOStreams
	ResolveRepository func(context.Context, string) (repository.Repository, error)
	APIClientForHost  func(string) (*dolthub.Client, error)
	Browser           browser.Browser
	Repository        string
	Number            int64
	Comments, Web     bool
	Exporter          cmdutil.Exporter
	client            client
}

func NewCmdView(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, ResolveRepository: f.ResolveRepository, APIClientForHost: f.APIClientForHost, Browser: f.Browser}
	if runF == nil {
		runF = viewRun
	}
	c := &cobra.Command{Use: "view NUMBER", Short: "View a pull request", Args: func(c *cobra.Command, args []string) error {
		if len(args) != 1 {
			return cmdutil.FlagErrorf("%s requires exactly one argument", c.CommandPath())
		}
		n, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil || n < 1 {
			return cmdutil.FlagErrorf("pull request number must be a positive integer")
		}
		o.Number = n
		return nil
	}, RunE: func(c *cobra.Command, _ []string) error { return runF(c.Context(), o) }}
	c.PreRunE = func(c *cobra.Command, _ []string) error {
		structured := c.Flags().Changed("json") || c.Flags().Changed("jq") || c.Flags().Changed("template")
		if o.Web && (o.Comments || structured) {
			return cmdutil.FlagErrorf("--web cannot be used with --comments or structured output")
		}
		return nil
	}
	cmdutil.AddDatabaseFlag(c, &o.Repository)
	c.Flags().BoolVarP(&o.Comments, "comments", "c", false, "View pull request comments")
	c.Flags().BoolVarP(&o.Web, "web", "w", false, "Open the pull request in a browser")
	cmdutil.AddJSONFlags(c, &o.Exporter, jsonFields)
	return c
}

func viewRun(ctx context.Context, o *Options) error {
	r, err := o.ResolveRepository(ctx, o.Repository)
	if err != nil {
		return err
	}
	if o.Web {
		return o.Browser.Browse(pullURL(r, o.Number))
	}
	c := o.client
	if c == nil {
		c, err = o.APIClientForHost(r.Host)
		if err != nil {
			return err
		}
	}
	pull, err := c.GetPull(ctx, r.Owner, r.Name, o.Number)
	if err != nil {
		return err
	}
	var comments []dolthub.PullComment
	if o.Comments {
		comments, err = c.ListPullComments(ctx, r.Owner, r.Name, o.Number)
		if err != nil {
			return err
		}
	}
	if o.Exporter != nil {
		return o.Exporter.Write(o.IO, pullOutput{Pull: pull, Comments: comments})
	}
	if err := renderPull(o.IO, pull); err != nil {
		return err
	}
	if !o.Comments {
		return nil
	}
	return renderComments(o.IO, comments)
}

type pullOutput struct {
	dolthub.Pull
	Comments []dolthub.PullComment `json:"comments,omitempty"`
}

func renderPull(io *iostreams.IOStreams, pull dolthub.Pull) error {
	t := tableprinter.New(io, "FIELD", "VALUE")
	rows := [][2]string{{"Title", pull.Title}, {"State", string(pull.State)}, {"Author", pull.Creator}, {"Number", strconv.FormatInt(pull.PullNumber, 10)}, {"From", branch(pull.FromBranch)}, {"Into", branch(pull.ToBranch)}, {"Created", pull.CreatedAt.Format("2006-01-02T15:04:05Z07:00")}, {"Description", fallback(pull.Description)}}
	for _, row := range rows {
		_ = t.AddRow(row[0], row[1])
	}
	return t.Render()
}

func renderComments(io *iostreams.IOStreams, comments []dolthub.PullComment) error {
	if len(comments) == 0 {
		return nil
	}
	t := tableprinter.New(io, "AUTHOR", "CREATED", "COMMENT")
	for _, c := range comments {
		_ = t.AddRow(c.Author, c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), c.Body)
	}
	return t.Render()
}

func branch(r dolthub.BranchRef) string {
	return r.Database.Owner + "/" + r.Database.Name + ":" + r.BranchName
}
func fallback(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
func pullURL(r repository.Repository, number int64) string {
	path := "/repositories/" + r.Owner + "/" + r.Name + "/pulls/" + strconv.FormatInt(number, 10)
	raw := "/repositories/" + url.PathEscape(r.Owner) + "/" + url.PathEscape(r.Name) + "/pulls/" + strconv.FormatInt(number, 10)
	return (&url.URL{Scheme: "https", Host: r.Host, Path: path, RawPath: raw}).String()
}
