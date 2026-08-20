// Package factory constructs production command dependencies.
package factory

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/dolthub/cli/internal/authflow"
	"github.com/dolthub/cli/internal/browser"
	"github.com/dolthub/cli/internal/config"
	"github.com/dolthub/cli/internal/credentials"
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/internal/httptransport"
	"github.com/dolthub/cli/internal/oauth"
	"github.com/dolthub/cli/internal/prompt"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

const OAuthClientIDEnv = "DH_OAUTH_CLIENT_ID"

// productionOAuthClientID is populated with -X for release builds once the
// shared, DoltHub-owned public OAuth application is registered. Development
// builds use DH_OAUTH_CLIENT_ID and never embed a personal application ID.
var productionOAuthClientID string

// New constructs the production command factory from process-level inputs.
func New(appVersion string, io *iostreams.IOStreams) *cmdutil.Factory {
	var once sync.Once
	var cfg config.Config
	var cfgErr error
	f := &cmdutil.Factory{
		AppVersion:  appVersion,
		IO:          io,
		Credentials: credentials.EnvironmentStore{Store: credentials.NewSystemStore(), LookupEnv: os.LookupEnv},
		Prompter:    prompt.System{IO: io},
		LookupEnv:   os.LookupEnv,
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
	f.RefreshToken = func(ctx context.Context, refreshToken string) (credentials.OAuthToken, error) {
		cfg, err := f.Config()
		if err != nil {
			return credentials.OAuthToken{}, err
		}
		client, err := oauthClient(appVersion, cfg.Host(), f.LookupEnv)
		if err != nil {
			return credentials.OAuthToken{}, err
		}
		return client.Refresh(ctx, refreshToken)
	}
	authenticator, err := authflow.NewBrowserAuthenticator(authflow.BrowserOptions{
		Protocol: func(host string) (authflow.OAuthProtocol, error) {
			return oauthClient(appVersion, host, f.LookupEnv)
		},
		Browser: browser.NewSystem(),
		Out:     io.Out,
	})
	if err != nil {
		panic(fmt.Sprintf("construct browser authenticator: %v", err))
	}
	f.Authenticator = authenticator
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
				refresh := func(ctx context.Context, refreshToken string) (credentials.OAuthToken, error) {
					client, err := oauthClient(appVersion, host, f.LookupEnv)
					if err != nil {
						return credentials.OAuthToken{}, err
					}
					return client.Refresh(ctx, refreshToken)
				}
				source := &credentials.TokenSource{Store: f.Credentials, Host: host, User: user, Refresh: refresh}
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

func oauthClient(appVersion, host string, lookupEnv func(string) (string, bool)) (*oauth.Client, error) {
	clientID, err := oauthClientID(lookupEnv)
	if err != nil {
		return nil, err
	}
	origin, err := webOrigin(host)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{Transport: httptransport.New(http.DefaultTransport, appVersion)}
	return oauth.NewClient(httpClient, origin, clientID)
}

func oauthClientID(lookupEnv func(string) (string, bool)) (string, error) {
	if lookupEnv != nil {
		if clientID, ok := lookupEnv(OAuthClientIDEnv); ok && strings.TrimSpace(clientID) != "" {
			return strings.TrimSpace(clientID), nil
		}
	}
	if strings.TrimSpace(productionOAuthClientID) != "" {
		return strings.TrimSpace(productionOAuthClientID), nil
	}
	return "", fmt.Errorf("OAuth client ID is not configured: set %s for development", OAuthClientIDEnv)
}

func webOrigin(host string) (*url.URL, error) {
	if host == "" || strings.ContainsAny(host, "/:?#@") {
		return nil, fmt.Errorf("invalid DoltHub host %q: expected a hostname", host)
	}
	return url.Parse("https://" + host)
}

func apiBaseURL(host string) (*url.URL, error) {
	origin, err := webOrigin(host)
	if err != nil {
		return nil, err
	}
	origin.Path = "/api/v2/"
	return origin, nil
}
