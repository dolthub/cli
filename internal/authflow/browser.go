package authflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dolthub/cli/internal/browser"
	"github.com/dolthub/cli/internal/credentials"
	"github.com/dolthub/cli/internal/oauth"
)

const (
	DefaultCallbackURL = "http://localhost:53682/callback"
	DefaultTimeout     = 5 * time.Minute
)

// OAuthProtocol is the portion of the OAuth client needed by browser login.
type OAuthProtocol interface {
	NewAuthorization(string) (oauth.Authorization, error)
	ExchangeCode(context.Context, string, string, string) (credentials.OAuthToken, error)
	CurrentUsername(context.Context, string) (string, error)
}

// ProtocolFactory constructs a host-specific OAuth client.
type ProtocolFactory func(host string) (OAuthProtocol, error)

// ListenerFactory exists so callback tests can bind an ephemeral port.
type ListenerFactory func(network, address string) (net.Listener, error)

// BrowserOptions configures one browser-based OAuth authenticator.
type BrowserOptions struct {
	Protocol    ProtocolFactory
	Browser     browser.Browser
	Out         io.Writer
	CallbackURL string
	Timeout     time.Duration
	Listen      ListenerFactory
}

// BrowserAuthenticator completes public-client OAuth through a loopback callback.
type BrowserAuthenticator struct {
	protocol    ProtocolFactory
	browser     browser.Browser
	out         io.Writer
	callbackURL *url.URL
	timeout     time.Duration
	listen      ListenerFactory
}

func NewBrowserAuthenticator(opts BrowserOptions) (*BrowserAuthenticator, error) {
	if opts.Protocol == nil {
		return nil, errors.New("oauth protocol factory is required")
	}
	if opts.Browser == nil {
		return nil, errors.New("browser is required")
	}
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.CallbackURL == "" {
		opts.CallbackURL = DefaultCallbackURL
	}
	callbackURL, err := validateCallbackURL(opts.CallbackURL)
	if err != nil {
		return nil, err
	}
	if opts.Timeout == 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.Timeout < 0 {
		return nil, errors.New("oauth callback timeout must be positive")
	}
	if opts.Listen == nil {
		opts.Listen = net.Listen
	}
	return &BrowserAuthenticator{protocol: opts.Protocol, browser: opts.Browser, out: opts.Out, callbackURL: callbackURL, timeout: opts.Timeout, listen: opts.Listen}, nil
}

func validateCallbackURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.User != nil || u.Path != "/callback" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("oauth callback URL must be an HTTP loopback /callback URL")
	}
	host := u.Hostname()
	if !strings.EqualFold(host, "localhost") {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return nil, errors.New("oauth callback URL must use a loopback host")
		}
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 0 || port > 65535 {
		return nil, errors.New("oauth callback URL must include a valid port")
	}
	copyURL := *u
	return &copyURL, nil
}

type callbackResult struct {
	code string
	err  error
}

// Login opens the authorization page, waits for the callback, exchanges its
// code, and validates the resulting identity. It does not persist credentials.
func (a *BrowserAuthenticator) Login(ctx context.Context, host string) (LoginResult, error) {
	protocol, err := a.protocol(host)
	if err != nil {
		return LoginResult{}, err
	}

	callbackURL := *a.callbackURL
	listenHost := callbackURL.Hostname()
	if strings.EqualFold(listenHost, "localhost") {
		listenHost = "127.0.0.1"
	}
	listener, err := a.listen("tcp", net.JoinHostPort(listenHost, callbackURL.Port()))
	if err != nil {
		return LoginResult{}, errors.New("start oauth callback listener")
	}
	defer listener.Close()
	if callbackURL.Port() == "0" {
		port := listener.Addr().(*net.TCPAddr).Port
		callbackURL.Host = net.JoinHostPort(callbackURL.Hostname(), strconv.Itoa(port))
	}
	redirectURI := callbackURL.String()

	authorization, err := protocol.NewAuthorization(redirectURI)
	if err != nil {
		return LoginResult{}, err
	}

	result := make(chan callbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != callbackURL.Path {
			http.Error(w, "Authorization could not be completed.", http.StatusNotFound)
			deliverCallback(result, callbackResult{err: errors.New("oauth callback used an unexpected path")})
			return
		}
		callbackHandler(authorization.State, result).ServeHTTP(w, r)
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer server.Close()
	go func() { _ = server.Serve(listener) }()

	if err := a.browser.Browse(authorization.URL.String()); err != nil {
		_, _ = fmt.Fprintf(a.out, "Could not open a browser. Visit this URL to continue:\n%s\n", authorization.URL.String())
	}

	waitCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	var callback callbackResult
	select {
	case callback = <-result:
	case <-waitCtx.Done():
		return LoginResult{}, waitCtx.Err()
	}
	if callback.err != nil {
		return LoginResult{}, callback.err
	}

	token, err := protocol.ExchangeCode(ctx, callback.code, redirectURI, authorization.CodeVerifier)
	if err != nil {
		return LoginResult{}, err
	}
	username, err := protocol.CurrentUsername(ctx, token.AccessToken)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Host: host, Username: username, Credential: token}, nil
}

func callbackHandler(expectedState string, result chan<- callbackResult) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			deliverCallback(result, callbackResult{err: errors.New("oauth callback used an unsupported method")})
			return
		}
		query := r.URL.Query()
		states := query["state"]
		if len(states) != 1 || states[0] == "" || states[0] != expectedState {
			http.Error(w, "Authorization could not be completed.", http.StatusBadRequest)
			deliverCallback(result, callbackResult{err: errors.New("oauth callback state mismatch")})
			return
		}
		if oauthErrors := query["error"]; len(oauthErrors) != 0 {
			code := "oauth_error"
			if len(oauthErrors) == 1 {
				code = safeCallbackErrorCode(oauthErrors[0])
			}
			http.Error(w, "Authorization was rejected.", http.StatusBadRequest)
			deliverCallback(result, callbackResult{err: fmt.Errorf("oauth authorization failed: %s", code)})
			return
		}
		codes := query["code"]
		if len(codes) != 1 || codes[0] == "" {
			http.Error(w, "Authorization could not be completed.", http.StatusBadRequest)
			deliverCallback(result, callbackResult{err: errors.New("oauth callback did not contain one authorization code")})
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "Authentication complete. You may close this window and return to dh.\n")
		deliverCallback(result, callbackResult{code: codes[0]})
	}
}

func deliverCallback(result chan<- callbackResult, value callbackResult) {
	select {
	case result <- value:
	default:
	}
}

func safeCallbackErrorCode(code string) string {
	if code == "" || len(code) > 64 {
		return "oauth_error"
	}
	for _, r := range code {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' && r != '-' {
			return "oauth_error"
		}
	}
	return code
}
