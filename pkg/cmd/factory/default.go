// Package factory constructs production command dependencies.
package factory

import (
	"context"
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
	"github.com/dolthub/cli/internal/repository"
	"github.com/dolthub/cli/pkg/cmdutil"
	"github.com/dolthub/cli/pkg/iostreams"
)

const (
	OAuthClientIDEnv = "DH_OAUTH_CLIENT_ID"
	productionHost   = "www.dolthub.com"
)

// productionOAuthClientID is the registered public DoltHub CLI application.
// Ordinary builds include it; custom builds may override it with -X.
var productionOAuthClientID = "dhoci.v1.nbdj3mjpe8sdes2c7i1i9ul3l2jm091h24o2s7f462aj3b55qsv0"

// New constructs the production command factory from process-level inputs.
func New(appVersion string, io *iostreams.IOStreams) *cmdutil.Factory {
	var once sync.Once
	var cfg config.Config
	var cfgErr error
	credentialPath, credentialPathErr := credentials.Path()
	fileStore := credentials.NewFileStore(credentialPath)
	if credentialPathErr != nil {
		fileStore = credentials.NewUnavailableFileStore(credentialPathErr)
	}
	systemPrompter := prompt.System{IO: io}
	f := &cmdutil.Factory{
		AppVersion:  appVersion,
		IO:          io,
		Credentials: credentials.NewFallbackStore(credentials.NewSystemStore(), fileStore),
		Prompter:    systemPrompter,
		LookupEnv:   os.LookupEnv,
		Browser:     browser.NewSystem(),
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
	f.APIClientForHost = func(host string) (*dolthub.Client, error) {
		cfg, err := f.Config()
		if err != nil {
			return nil, err
		}
		transport := httptransport.New(http.DefaultTransport, appVersion)
		if envToken, ok := f.LookupEnv("DH_TOKEN"); ok && envToken != "" {
			transport, err = httptransport.NewAuthenticated(http.DefaultTransport, appVersion, host, envToken)
		} else if user, ok := cfg.ActiveUser(host); ok {
			source := &credentials.TokenSource{Store: f.Credentials, Host: host, User: user, Refresh: func(ctx context.Context, refreshToken string) (credentials.OAuthToken, error) {
				client, err := oauthClient(appVersion, host, f.LookupEnv)
				if err != nil {
					return credentials.OAuthToken{}, err
				}
				return client.Refresh(ctx, refreshToken)
			}}
			transport, err = httptransport.NewAuthenticatedTokenSource(http.DefaultTransport, appVersion, host, source)
		}
		if err != nil {
			return nil, err
		}
		client := &http.Client{Transport: transport}
		base, err := apiBaseURL(host)
		if err != nil {
			return nil, err
		}
		return dolthub.NewClient(client, base)
	}
	f.ResolveRepository = func(ctx context.Context, explicit string) (repository.Repository, error) {
		cfg, err := f.Config()
		if err != nil {
			return repository.Repository{}, err
		}
		resolver := repository.Resolver{
			Host:      cfg.Host,
			LookupEnv: f.LookupEnv,
			Configured: func() (repository.Repository, bool) {
				if f.LookupEnv != nil {
					if value, ok := f.LookupEnv("DH_REPO"); ok && strings.TrimSpace(value) != "" {
						return repository.Repository{}, false
					}
				}
				return cfg.DefaultRepository()
			},
			Remotes:   repository.ReadDoltRemotes,
			CanPrompt: io.IsStdinTTY,
			Select:    systemPrompter.Select,
		}
		return resolver.Resolve(ctx, explicit)
	}
	return f
}

func oauthClient(appVersion, host string, lookupEnv func(string) (string, bool)) (*oauth.Client, error) {
	clientID, err := oauthClientID(host, lookupEnv)
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

func oauthClientID(host string, lookupEnv func(string) (string, bool)) (string, error) {
	if lookupEnv != nil {
		if clientID, ok := lookupEnv(OAuthClientIDEnv); ok && strings.TrimSpace(clientID) != "" {
			return strings.TrimSpace(clientID), nil
		}
	}
	if strings.EqualFold(strings.TrimSpace(host), productionHost) && strings.TrimSpace(productionOAuthClientID) != "" {
		return strings.TrimSpace(productionOAuthClientID), nil
	}
	return "", fmt.Errorf("OAuth client ID is not configured for %s; set %s for this host", host, OAuthClientIDEnv)
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
