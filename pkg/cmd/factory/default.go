// Package factory constructs production command dependencies.
package factory

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/dolthub/cli/internal/authflow"
	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/credentials"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/httptransport"
	"github.com/dolthub/cli/internal/prompt"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

// New constructs the production command factory from process-level inputs.
func New(appVersion string, io *iostreams.IOStreams) *cmdutil.Factory {
	var once sync.Once
	var cfg config.Config
	var cfgErr error
	f := &cmdutil.Factory{
		AppVersion:    appVersion,
		IO:            io,
		Credentials:   credentials.EnvironmentStore{Store: credentials.NewSystemStore(), LookupEnv: os.LookupEnv},
		Authenticator: authflow.Unavailable{},
		Prompter:      prompt.System{IO: io},
		LookupEnv:     os.LookupEnv,
	}
	f.Config = func() (config.Config, error) {
		once.Do(func() {
			path, err := config.Path()
			if err != nil {
				cfgErr = err
				return
			}
			var file *config.File
			file, cfgErr = config.Load(path)
			if cfgErr == nil {
				cfg = config.Environment{Config: file, LookupEnv: os.LookupEnv}
			}
		})
		return cfg, cfgErr
	}
	f.HTTPClient = func() (*http.Client, error) {
		cfg, err := f.Config()
		if err != nil {
			return nil, err
		}
		host := cfg.Host()
		transport := httptransport.New(http.DefaultTransport, appVersion)
		if envToken, ok := f.LookupEnv("DH_TOKEN"); ok && envToken != "" {
			transport, err = httptransport.NewAuthenticated(http.DefaultTransport, appVersion, host, envToken)
			if err != nil {
				return nil, err
			}
		} else if user, ok := cfg.ActiveUser(host); ok {
			_, tokenErr := credentials.GetOAuthToken(f.Credentials, host, user)
			if tokenErr != nil && !errors.Is(tokenErr, credentials.ErrNotFound) {
				return nil, fmt.Errorf("load credential: %w", tokenErr)
			}
			if tokenErr == nil {
				source := &credentials.TokenSource{Store: f.Credentials, Host: host, User: user, Refresh: f.RefreshToken}
				transport, err = httptransport.NewAuthenticatedTokenSource(http.DefaultTransport, appVersion, host, source)
				if err != nil {
					return nil, err
				}
			}
		}
		return &http.Client{Transport: transport}, nil
	}
	f.APIClient = func() (*dolthub.Client, error) {
		cfg, err := f.Config()
		if err != nil {
			return nil, err
		}
		client, err := f.HTTPClient()
		if err != nil {
			return nil, err
		}
		base, err := apiBaseURL(cfg.Host())
		if err != nil {
			return nil, err
		}
		return dolthub.NewClient(client, base)
	}
	return f
}

func apiBaseURL(host string) (*url.URL, error) {
	if host == "" || strings.ContainsAny(host, "/:?#@") {
		return nil, fmt.Errorf("invalid DoltHub host %q: expected a hostname", host)
	}
	return url.Parse("https://" + host + "/api/v2/")
}
