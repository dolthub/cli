package browse

import (
	"context"
	"net/url"
	"strconv"

	"github.com/dolthub/cli/internal/browser"
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/spf13/cobra"
)

type Options struct {
	ResolveRepository func(context.Context, string) (repository.Repository, error)
	Browser           browser.Browser
	Repository        string
	Pull              int
	Branch            string
	positionalPull    int
}

func NewCmdBrowse(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	opts := &Options{ResolveRepository: f.ResolveRepository, Browser: f.Browser}
	if runF == nil {
		runF = browseRun
	}
	cmd := &cobra.Command{
		Use:   "browse [NUMBER]",
		Short: "Open a database repository in the browser",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return cmdutil.FlagErrorf("%s accepts at most one argument", cmd.CommandPath())
			}
			if len(args) == 1 {
				number, err := strconv.Atoi(args[0])
				if err != nil || number <= 0 {
					return cmdutil.FlagErrorf("pull request number must be a positive integer")
				}
				opts.positionalPull = number
			}
			selectors := 0
			if opts.positionalPull > 0 {
				selectors++
			}
			if cmd.Flags().Changed("pull") {
				selectors++
				if opts.Pull <= 0 {
					return cmdutil.FlagErrorf("--pull must be a positive integer")
				}
			}
			if cmd.Flags().Changed("branch") {
				selectors++
				if opts.Branch == "" {
					return cmdutil.FlagErrorf("--branch must not be empty")
				}
			}
			if selectors > 1 {
				return cmdutil.FlagErrorf("pull request and branch selectors are mutually exclusive")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error { return runF(cmd.Context(), opts) },
	}
	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Select a database repository using [HOST/]OWNER/REPO")
	cmd.Flags().IntVar(&opts.Pull, "pull", 0, "Open a pull request by number")
	cmd.Flags().StringVar(&opts.Branch, "branch", "", "Open a branch")
	return cmd
}

func browseRun(ctx context.Context, opts *Options) error {
	repo, err := opts.ResolveRepository(ctx, opts.Repository)
	if err != nil {
		return err
	}
	segments := []string{"repositories", repo.Owner, repo.Name}
	if opts.Branch != "" {
		segments = append(segments, "data", opts.Branch)
	} else if number := selectedPull(opts); number > 0 {
		segments = append(segments, "pulls", strconv.Itoa(number))
	}
	return opts.Browser.Browse(webURL(repo.Host, segments...))
}

func selectedPull(opts *Options) int {
	if opts.Pull > 0 {
		return opts.Pull
	}
	return opts.positionalPull
}

func webURL(host string, segments ...string) string {
	path, rawPath := "", ""
	for _, segment := range segments {
		path += "/" + segment
		rawPath += "/" + url.PathEscape(segment)
	}
	return (&url.URL{Scheme: "https", Host: host, Path: path, RawPath: rawPath}).String()
}
