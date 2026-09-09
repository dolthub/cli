package comment

import (
	"context"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/internal/tableprinter"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
)

var jsonFields = []string{"author", "body", "comment_id", "created_at", "updated_at"}

type client interface {
	CreatePullComment(context.Context, string, string, int64, dolthub.CreatePullCommentRequest) (dolthub.PullComment, error)
}
type Options struct {
	IO                *iostreams.IOStreams
	ResolveRepository func(context.Context, string) (repository.Repository, error)
	APIClientForHost  func(string) (*dolthub.Client, error)
	Repository        string
	Number            int64
	Body, BodyFile    string
	Exporter          cmdutil.Exporter
	client            client
}

func NewCmdComment(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, ResolveRepository: f.ResolveRepository, APIClientForHost: f.APIClientForHost}
	if runF == nil {
		runF = commentRun
	}
	c := &cobra.Command{Use: "comment NUMBER", Short: "Add a pull request comment", Args: func(c *cobra.Command, args []string) error {
		if err := cmdutil.ExactArgs(1)(c, args); err != nil {
			return err
		}
		n, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil || n < 1 {
			return cmdutil.FlagErrorf("pull request number must be a positive integer")
		}
		o.Number = n
		return nil
	}, RunE: func(c *cobra.Command, _ []string) error { return runF(c.Context(), o) }}
	c.PreRunE = func(c *cobra.Command, _ []string) error {
		if c.Flags().Changed("body") && c.Flags().Changed("body-file") {
			return cmdutil.FlagErrorf("--body and --body-file are mutually exclusive")
		}
		if !o.IO.IsStdinTTY() && !c.Flags().Changed("body") && !c.Flags().Changed("body-file") {
			return cmdutil.FlagErrorf("--body or --body-file is required in non-interactive use")
		}
		return nil
	}
	cmdutil.AddDatabaseFlag(c, &o.Repository)
	c.Flags().StringVarP(&o.Body, "body", "b", "", "Comment body")
	c.Flags().StringVarP(&o.BodyFile, "body-file", "F", "", "Read comment body from file")
	cmdutil.AddJSONFlags(c, &o.Exporter, jsonFields)
	return cmdutil.WithDocs(c, "dh pr comment 1 --db OWNER/people --body \"Ready for review\"", cmdutil.DocMetadata{
		Arguments:   []cmdutil.DocArgument{{Name: "NUMBER", Description: "Positive pull request number in the selected database.", Optional: false}},
		Constraints: []string{"Comment body must not be empty. Noninteractive use requires --body or --body-file; interactive use can prompt. The flags are mutually exclusive. --body-file - reads stdin."},
		Output:      "Prints the comment author, creation time, and body, or selected JSON fields.",
	})
}
func commentRun(ctx context.Context, o *Options) error {
	body, err := cmdutil.ReadTextSource(o.Body, o.BodyFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" && o.IO.IsStdinTTY() {
		body, err = cmdutil.PromptLine(o.IO, "Comment")
		if err != nil {
			return err
		}
	}
	if strings.TrimSpace(body) == "" {
		return cmdutil.FlagErrorf("comment body must not be empty")
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
	comment, err := c.CreatePullComment(ctx, r.Owner, r.Name, o.Number, dolthub.CreatePullCommentRequest{Body: body})
	if err != nil {
		return err
	}
	if o.Exporter != nil {
		return o.Exporter.Write(o.IO, comment)
	}
	t := tableprinter.New(o.IO, "AUTHOR", "CREATED", "COMMENT")
	_ = t.AddRow(comment.Author, comment.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), comment.Body)
	return t.Render()
}
