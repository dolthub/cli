package create

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

var jsonFields = []string{"created_at", "creator", "description", "from_branch", "pull_number", "state", "title", "to_branch"}

type client interface {
	CreatePull(context.Context, string, string, dolthub.CreatePullRequest) (dolthub.Pull, error)
}
type Options struct {
	IO                                            *iostreams.IOStreams
	ResolveRepository                             func(context.Context, string) (repository.Repository, error)
	APIClientForHost                              func(string) (*dolthub.Client, error)
	Repository, Title, Body, BodyFile, Head, Base string
	Exporter                                      cmdutil.Exporter
	client                                        client
}

func NewCmdCreate(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	o := &Options{IO: f.IO, ResolveRepository: f.ResolveRepository, APIClientForHost: f.APIClientForHost}
	if runF == nil {
		runF = createRun
	}
	c := &cobra.Command{Use: "create", Short: "Create a pull request", Args: cmdutil.NoArgs, RunE: func(c *cobra.Command, _ []string) error { return runF(c.Context(), o) }}
	c.PreRunE = func(c *cobra.Command, _ []string) error {
		if c.Flags().Changed("body") && c.Flags().Changed("body-file") {
			return cmdutil.FlagErrorf("--body and --body-file are mutually exclusive")
		}
		if !o.IO.IsStdinTTY() {
			if strings.TrimSpace(o.Title) == "" {
				return cmdutil.FlagErrorf("--title is required in non-interactive use")
			}
			if strings.TrimSpace(o.Head) == "" {
				return cmdutil.FlagErrorf("--head is required in non-interactive use")
			}
			if strings.TrimSpace(o.Base) == "" {
				return cmdutil.FlagErrorf("--base is required in non-interactive use")
			}
		}
		return nil
	}
	cmdutil.AddDatabaseFlag(c, &o.Repository)
	c.Flags().StringVarP(&o.Title, "title", "t", "", "Pull request title")
	c.Flags().StringVarP(&o.Body, "body", "b", "", "Pull request body")
	c.Flags().StringVarP(&o.BodyFile, "body-file", "F", "", "Read pull request body from file")
	c.Flags().StringVarP(&o.Head, "head", "H", "", "Source [OWNER/DB:]BRANCH")
	c.Flags().StringVarP(&o.Base, "base", "B", "", "Target branch")
	cmdutil.AddJSONFlags(c, &o.Exporter, jsonFields)
	return cmdutil.WithDocs(c, "dh pr create --db OWNER/people --head feature/people --base main --title \"Update people\"", cmdutil.DocMetadata{
		Constraints: []string{"Noninteractive use requires --title, --head, and --base; interactive use prompts for missing values.", "--head accepts BRANCH or OWNER/DB:BRANCH for a cross-database source. --base is a branch in the selected target database.", "--body and --body-file are mutually exclusive; --body-file - reads stdin."},
		Output:      "Prints the created pull request number, title, state, and source/target branches, or selected JSON fields.",
	})
}
func createRun(ctx context.Context, o *Options) error {
	r, err := o.ResolveRepository(ctx, o.Repository)
	if err != nil {
		return err
	}
	title := strings.TrimSpace(o.Title)
	head := strings.TrimSpace(o.Head)
	base := strings.TrimSpace(o.Base)
	if title == "" {
		title, err = cmdutil.PromptLine(o.IO, "Title")
		if err != nil {
			return err
		}
	}
	if head == "" {
		head, err = cmdutil.PromptLine(o.IO, "Head branch")
		if err != nil {
			return err
		}
	}
	if base == "" {
		base, err = cmdutil.PromptLine(o.IO, "Base branch")
		if err != nil {
			return err
		}
	}
	if title == "" || head == "" || base == "" {
		return cmdutil.FlagErrorf("title, head, and base must not be empty")
	}
	from, err := parseHead(head, r)
	if err != nil {
		return err
	}
	body, err := cmdutil.ReadTextSource(o.Body, o.BodyFile)
	if err != nil {
		return err
	}
	var description *string
	if body != "" {
		description = &body
	}
	request := dolthub.CreatePullRequest{Title: title, Description: description, FromBranch: from, ToBranch: dolthub.BranchRef{Database: dolthub.DatabaseRef{Owner: r.Owner, Name: r.Name}, BranchName: base}}
	c := o.client
	if c == nil {
		c, err = o.APIClientForHost(r.Host)
		if err != nil {
			return err
		}
	}
	pull, err := c.CreatePull(ctx, r.Owner, r.Name, request)
	if err != nil {
		return err
	}
	if o.Exporter != nil {
		return o.Exporter.Write(o.IO, pull)
	}
	t := tableprinter.New(o.IO, "NUMBER", "TITLE", "STATE", "FROM", "INTO")
	_ = t.AddRow(strconv.FormatInt(pull.PullNumber, 10), pull.Title, string(pull.State), branch(pull.FromBranch), branch(pull.ToBranch))
	return t.Render()
}
func parseHead(value string, r repository.Repository) (dolthub.BranchRef, error) {
	parts := strings.Split(value, ":")
	if len(parts) == 1 {
		return dolthub.BranchRef{Database: dolthub.DatabaseRef{Owner: r.Owner, Name: r.Name}, BranchName: parts[0]}, nil
	}
	if len(parts) != 2 || parts[1] == "" {
		return dolthub.BranchRef{}, cmdutil.FlagErrorf("--head must be BRANCH or OWNER/DB:BRANCH")
	}
	db := strings.Split(parts[0], "/")
	if len(db) != 2 || db[0] == "" || db[1] == "" {
		return dolthub.BranchRef{}, cmdutil.FlagErrorf("--head database must be OWNER/DB")
	}
	return dolthub.BranchRef{Database: dolthub.DatabaseRef{Owner: db[0], Name: db[1]}, BranchName: parts[1]}, nil
}
func branch(r dolthub.BranchRef) string {
	return r.Database.Owner + "/" + r.Database.Name + ":" + r.BranchName
}
