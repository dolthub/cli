package status

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/credentials"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/httptransport"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

type Options struct {
	IO            *iostreams.IOStreams
	Config        func() (config.Config, error)
	Credentials   credentials.Store
	RefreshToken  credentials.RefreshFunc
	LookupEnv     func(string) (string, bool)
	AppVersion    string
	BaseTransport http.RoundTripper
}

func NewCmdStatus(f *cmdutil.Factory, runF func(context.Context, *Options) error) *cobra.Command {
	opts := &Options{IO: f.IO, Config: f.Config, Credentials: f.Credentials, RefreshToken: f.RefreshToken, LookupEnv: f.LookupEnv, AppVersion: f.AppVersion}
	if runF == nil {
		runF = statusRun
	}
	return &cobra.Command{Use: "status", Short: "View authentication status", Args: cmdutil.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return runF(cmd.Context(), opts) }}
}
func statusRun(ctx context.Context, opts *Options) error {
	cfg, err := opts.Config()
	if err != nil {
		return err
	}
	host := cfg.Host()
	user, hasUser := cfg.ActiveUser(host)
	token, fromEnv := opts.LookupEnv("DH_TOKEN")
	source := "credential store"
	if !fromEnv || token == "" {
		if !hasUser {
			return &cmdutil.AuthError{}
		}
		_, err = credentials.GetOAuthToken(opts.Credentials, host, user)
		if err != nil {
			if errors.Is(err, credentials.ErrNotFound) {
				return &cmdutil.AuthError{}
			}
			return err
		}
	} else {
		source = "DH_TOKEN"
	}
	base := opts.BaseTransport
	if base == nil {
		base = http.DefaultTransport
	}
	var transport http.RoundTripper
	if source == "DH_TOKEN" {
		credential, decodeErr := credentials.DecodeOAuthToken(token)
		if decodeErr != nil {
			return &cmdutil.AuthError{Err: fmt.Errorf("authentication for %s is invalid", host)}
		}
		transport, err = httptransport.NewAuthenticated(base, opts.AppVersion, host, credential.AccessToken)
	} else {
		tokenSource := &credentials.TokenSource{Store: opts.Credentials, Host: host, User: user, Refresh: opts.RefreshToken}
		transport, err = httptransport.NewAuthenticatedTokenSource(base, opts.AppVersion, host, tokenSource)
	}
	if err != nil {
		return err
	}
	apiBase, _ := url.Parse("https://" + host + "/api/v2/")
	client, _ := dolthub.NewClient(&http.Client{Transport: transport}, apiBase)
	current, err := client.CurrentUser(ctx)
	if err != nil {
		return &cmdutil.AuthError{Err: fmt.Errorf("authentication for %s is invalid", host)}
	}
	_, err = fmt.Fprintf(opts.IO.Out, "Logged in to %s as %s (%s)\n", host, current.Username, source)
	return err
}
