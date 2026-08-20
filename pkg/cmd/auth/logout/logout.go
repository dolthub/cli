package logout

import (
	"context"
	"errors"
	"fmt"

	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/credentials"
	"github.com/dolthub/cli/internal/prompt"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

type Options struct {
	IO          *iostreams.IOStreams
	Config      func() (config.Config, error)
	Credentials credentials.Store
	Prompter    prompt.Prompter
	LookupEnv   func(string) (string, bool)
	Host        string
	Yes         bool
}

func NewCmdLogout(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	opts := &Options{IO: f.IO, Config: f.Config, Credentials: f.Credentials, Prompter: f.Prompter, LookupEnv: f.LookupEnv}
	if runF == nil {
		runF = logoutRun
	}
	cmd := &cobra.Command{Use: "logout", Short: "Log out of DoltHub", Args: cmdutil.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return runF(cmd.Context(), opts) }}
	cmd.Flags().StringVar(&opts.Host, "hostname", "", "DoltHub hostname")
	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false, "Skip confirmation")
	return cmd
}
func logoutRun(_ context.Context, opts *Options) error {
	if token, ok := opts.LookupEnv("DH_TOKEN"); ok && token != "" {
		return errors.New("cannot log out while DH_TOKEN is set; unset the environment variable instead")
	}
	cfg, err := opts.Config()
	if err != nil {
		return err
	}
	host := opts.Host
	if host == "" {
		host = cfg.Host()
	}
	user, ok := cfg.ActiveUser(host)
	if !ok {
		return &cmdutil.AuthError{Err: fmt.Errorf("not logged in to %s", host)}
	}
	if !opts.Yes && opts.IO.IsStdinTTY() {
		confirmed, err := opts.Prompter.Confirm(fmt.Sprintf("Log out of %s as %s?", host, user), false)
		if err != nil {
			return err
		}
		if !confirmed {
			return &cmdutil.CancelError{}
		}
	}
	stored, tokenErr := credentials.GetStored(opts.Credentials, host, user)
	if tokenErr != nil && !errors.Is(tokenErr, credentials.ErrNotFound) {
		return tokenErr
	}
	if tokenErr == nil {
		if err := opts.Credentials.Delete(host, user); err != nil && !errors.Is(err, credentials.ErrNotFound) {
			return err
		}
	}
	cfg.UnsetActiveUser(host)
	if err := cfg.Write(); err != nil {
		cfg.SetActiveUser(host, user)
		if tokenErr == nil {
			if restoreErr := credentials.SetAt(opts.Credentials, stored.Source, host, user, stored.Secret); restoreErr != nil {
				return errors.Join(err, fmt.Errorf("restore credential after config failure: %w", restoreErr))
			}
		}
		return err
	}
	_, err = fmt.Fprintf(opts.IO.Out, "Logged out of %s\n", host)
	return err
}
