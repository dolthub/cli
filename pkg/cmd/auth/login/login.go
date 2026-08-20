package login

import (
	"context"
	"errors"
	"fmt"

	"github.com/dolthub/cli/internal/authflow"
	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/credentials"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

type Options struct {
	IO            *iostreams.IOStreams
	Config        func() (config.Config, error)
	Credentials   credentials.Store
	Authenticator authflow.Authenticator
	LookupEnv     func(string) (string, bool)
	Host          string
}

func NewCmdLogin(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	opts := &Options{IO: f.IO, Config: f.Config, Credentials: f.Credentials, Authenticator: f.Authenticator, LookupEnv: f.LookupEnv}
	if runF == nil {
		runF = loginRun
	}
	cmd := &cobra.Command{Use: "login", Short: "Log in to DoltHub in a web browser", Args: cmdutil.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return runF(cmd.Context(), opts) }}
	cmd.Flags().StringVar(&opts.Host, "hostname", "", "DoltHub hostname")
	return cmd
}
func loginRun(ctx context.Context, opts *Options) error {
	if token, ok := opts.LookupEnv("DH_TOKEN"); ok && token != "" {
		return errors.New("cannot log in while DH_TOKEN is set; unset the environment variable first")
	}
	cfg, err := opts.Config()
	if err != nil {
		return err
	}
	host := opts.Host
	if host == "" {
		host = cfg.Host()
	}
	result, err := opts.Authenticator.Login(ctx, host)
	if err != nil {
		return err
	}
	if result.Host != host || result.Username == "" || result.Credential.Validate() != nil {
		return errors.New("browser login returned an invalid identity")
	}
	previousUser, replacing := cfg.ActiveUser(host)
	var previousToken string
	var hadPreviousToken bool
	if replacing && previousUser == result.Username {
		previousToken, err = opts.Credentials.Get(host, previousUser)
		if err == nil {
			hadPreviousToken = true
		} else if !errors.Is(err, credentials.ErrNotFound) {
			return fmt.Errorf("load existing credential: %w", err)
		}
	}
	encoded, err := credentials.EncodeOAuthToken(result.Credential)
	if err != nil {
		return errors.New("browser login returned an invalid credential")
	}
	if err := opts.Credentials.Set(host, result.Username, encoded); err != nil {
		return err
	}
	cfg.SetActiveUser(host, result.Username)
	if err := cfg.Write(); err != nil {
		if replacing {
			cfg.SetActiveUser(host, previousUser)
		} else {
			cfg.UnsetActiveUser(host)
		}
		rollbackErr := rollbackCredential(opts.Credentials, host, result.Username, previousToken, hadPreviousToken)
		return errors.Join(fmt.Errorf("write authentication config: %w", err), rollbackErr)
	}
	if replacing && previousUser != result.Username {
		if err := opts.Credentials.Delete(host, previousUser); err != nil && !errors.Is(err, credentials.ErrNotFound) {
			return fmt.Errorf("remove previous credential: %w", err)
		}
	}
	_, err = fmt.Fprintf(opts.IO.Out, "Logged in to %s as %s\n", host, result.Username)
	return err
}

func rollbackCredential(store credentials.Store, host, user, previousToken string, hadPreviousToken bool) error {
	if hadPreviousToken {
		if err := store.Set(host, user, previousToken); err != nil {
			return fmt.Errorf("restore previous credential: %w", err)
		}
		return nil
	}
	if err := store.Delete(host, user); err != nil && !errors.Is(err, credentials.ErrNotFound) {
		return fmt.Errorf("remove new credential: %w", err)
	}
	return nil
}
