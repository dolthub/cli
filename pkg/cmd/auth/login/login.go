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
	var previousCredential credentials.Stored
	var hadPreviousToken bool
	if replacing {
		previousCredential, err = credentials.GetStored(opts.Credentials, host, previousUser)
		if err == nil {
			hadPreviousToken = true
		} else if !errors.Is(err, credentials.ErrNotFound) && !errors.Is(err, credentials.ErrCredentialStoreUnavailable) {
			return fmt.Errorf("load existing credential: %w", err)
		}
	}
	storageSource, err := credentials.SetOAuthTokenPreferred(opts.Credentials, host, result.Username, result.Credential)
	if err != nil {
		return err
	}
	cfg.SetActiveUser(host, result.Username)
	if err := cfg.Write(); err != nil {
		if replacing {
			cfg.SetActiveUser(host, previousUser)
		} else {
			cfg.UnsetActiveUser(host)
		}
		rollbackErr := rollbackCredential(opts.Credentials, storageSource, host, result.Username, previousCredential, hadPreviousToken && previousUser == result.Username)
		return errors.Join(fmt.Errorf("write authentication config: %w", err), rollbackErr)
	}
	if replacing && previousUser != result.Username {
		if hadPreviousToken {
			if err := credentials.DeleteAt(opts.Credentials, previousCredential.Source, host, previousUser); err != nil && !errors.Is(err, credentials.ErrNotFound) {
				return fmt.Errorf("remove previous credential: %w", err)
			}
		}
	}
	if storageSource == credentials.SourceFile {
		if path, ok := credentials.FallbackFilePath(opts.Credentials); ok {
			_, _ = fmt.Fprintf(opts.IO.ErrOut, "warning: system credential storage is unavailable; authentication credentials were saved unencrypted to %s\n", path)
		} else {
			_, _ = fmt.Fprintln(opts.IO.ErrOut, "warning: system credential storage is unavailable; authentication credentials were saved unencrypted")
		}
	}
	_, err = fmt.Fprintf(opts.IO.Out, "Logged in to %s as %s\n", host, result.Username)
	return err
}

func rollbackCredential(store credentials.Store, newSource credentials.Source, host, user string, previous credentials.Stored, hadPreviousToken bool) error {
	if hadPreviousToken {
		if err := credentials.SetAt(store, previous.Source, host, user, previous.Secret); err != nil {
			return fmt.Errorf("restore previous credential: %w", err)
		}
		return nil
	}
	if err := credentials.DeleteAt(store, newSource, host, user); err != nil && !errors.Is(err, credentials.ErrNotFound) {
		return fmt.Errorf("remove new credential: %w", err)
	}
	return nil
}
