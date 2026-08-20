package authflow

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dolthub/cli/internal/credentials"
	"github.com/dolthub/cli/internal/oauth"
)

const testCode = "authorization-code-secret"

type fakeProtocol struct {
	mu           sync.Mutex
	redirectURI  string
	exchanged    bool
	exchangeCode string
	token        credentials.OAuthToken
	username     string
	exchangeErr  error
	identityErr  error
}

func (f *fakeProtocol) NewAuthorization(redirectURI string) (oauth.Authorization, error) {
	f.mu.Lock()
	f.redirectURI = redirectURI
	f.mu.Unlock()
	u, _ := url.Parse("https://example.test/oauth/authorize")
	query := u.Query()
	query.Set("redirect_uri", redirectURI)
	u.RawQuery = query.Encode()
	return oauth.Authorization{URL: u, State: "expected-state", CodeVerifier: "pkce-verifier"}, nil
}

func (f *fakeProtocol) ExchangeCode(_ context.Context, code, redirectURI, verifier string) (credentials.OAuthToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.exchanged = true
	f.exchangeCode = code
	if redirectURI != f.redirectURI || verifier != "pkce-verifier" {
		return credentials.OAuthToken{}, errors.New("incorrect exchange arguments")
	}
	return f.token, f.exchangeErr
}

func (f *fakeProtocol) CurrentUsername(context.Context, string) (string, error) {
	return f.username, f.identityErr
}

func (f *fakeProtocol) wasExchanged() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.exchanged
}

type fakeBrowser struct {
	browse func(string) error
}

func (f fakeBrowser) Browse(rawURL string) error { return f.browse(rawURL) }

func callbackBrowser(t *testing.T, callbackQuery string) fakeBrowser {
	t.Helper()
	return fakeBrowser{browse: func(rawURL string) error {
		authorizeURL, err := url.Parse(rawURL)
		if err != nil {
			t.Errorf("parse authorization URL: %v", err)
			return nil
		}
		callbackURL := authorizeURL.Query().Get("redirect_uri") + "?" + callbackQuery
		resp, err := http.Get(callbackURL) //nolint:gosec // callback URL is a test-only loopback address
		if err != nil {
			t.Errorf("request callback: %v", err)
			return nil
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}}
}

func newTestAuthenticator(t *testing.T, protocol *fakeProtocol, browser fakeBrowser, out io.Writer, timeout time.Duration) *BrowserAuthenticator {
	t.Helper()
	authenticator, err := NewBrowserAuthenticator(BrowserOptions{
		Protocol: func(string) (OAuthProtocol, error) { return protocol, nil },
		Browser:  browser, Out: out, CallbackURL: "http://localhost:0/callback", Timeout: timeout,
	})
	if err != nil {
		t.Fatal(err)
	}
	return authenticator
}

func TestBrowserLoginCompletesAndValidatesIdentity(t *testing.T) {
	token := credentials.OAuthToken{AccessToken: "access-token-secret", RefreshToken: "refresh-token-secret", TokenType: "Bearer", ExpiresAt: time.Now().Add(time.Hour)}
	protocol := &fakeProtocol{token: token, username: "alice"}
	authenticator := newTestAuthenticator(t, protocol, callbackBrowser(t, "code="+testCode+"&state=expected-state"), io.Discard, time.Second)

	result, err := authenticator.Login(context.Background(), "dev.dolthub.test")
	if err != nil {
		t.Fatal(err)
	}
	if result.Host != "dev.dolthub.test" || result.Username != "alice" || result.Credential.AccessToken != token.AccessToken {
		t.Fatalf("result = %#v", result)
	}
	if !protocol.wasExchanged() || protocol.exchangeCode != testCode {
		t.Fatal("authorization code was not exchanged")
	}
}

func TestBrowserFailurePrintsManualURLAndStillCompletes(t *testing.T) {
	protocol := &fakeProtocol{token: credentials.OAuthToken{AccessToken: "token"}, username: "alice"}
	var out bytes.Buffer
	browser := fakeBrowser{browse: func(rawURL string) error {
		u, _ := url.Parse(rawURL)
		callbackURL := u.Query().Get("redirect_uri") + "?code=" + testCode + "&state=expected-state"
		go func() {
			resp, err := http.Get(callbackURL) //nolint:gosec // test-only loopback callback
			if err == nil {
				resp.Body.Close()
			}
		}()
		return errors.New("no browser")
	}}
	authenticator := newTestAuthenticator(t, protocol, browser, &out, time.Second)
	if _, err := authenticator.Login(context.Background(), "www.dolthub.com"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "https://example.test/oauth/authorize") {
		t.Fatalf("manual output = %q", out.String())
	}
	if strings.Contains(out.String(), testCode) || strings.Contains(out.String(), "token") {
		t.Fatalf("manual output contains credential: %q", out.String())
	}
}

func TestBrowserLoginRejectsInvalidCallbacksWithoutExchange(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{name: "state mismatch", query: "code=" + testCode + "&state=wrong", want: "state mismatch"},
		{name: "missing state", query: "code=" + testCode, want: "state mismatch"},
		{name: "duplicate state", query: "code=" + testCode + "&state=expected-state&state=expected-state", want: "state mismatch"},
		{name: "missing code", query: "state=expected-state", want: "one authorization code"},
		{name: "duplicate code", query: "code=a&code=b&state=expected-state", want: "one authorization code"},
		{name: "rejected", query: "error=access_denied&state=expected-state", want: "access_denied"},
		{name: "unsafe error", query: "error=secret%3Dcredential&state=expected-state", want: "oauth_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			protocol := &fakeProtocol{}
			authenticator := newTestAuthenticator(t, protocol, callbackBrowser(t, tt.query), io.Discard, time.Second)
			_, err := authenticator.Login(context.Background(), "www.dolthub.com")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v", err)
			}
			if strings.Contains(err.Error(), "credential") || protocol.wasExchanged() {
				t.Fatalf("unsafe error or unexpected exchange: %v", err)
			}
		})
	}
}

func TestBrowserLoginTimeoutAndCancellation(t *testing.T) {
	neverCallsBack := fakeBrowser{browse: func(string) error { return nil }}
	t.Run("timeout", func(t *testing.T) {
		protocol := &fakeProtocol{}
		authenticator := newTestAuthenticator(t, protocol, neverCallsBack, io.Discard, 20*time.Millisecond)
		_, err := authenticator.Login(context.Background(), "www.dolthub.com")
		if !errors.Is(err, context.DeadlineExceeded) || protocol.wasExchanged() {
			t.Fatalf("error = %v, exchanged = %v", err, protocol.wasExchanged())
		}
	})
	t.Run("cancellation", func(t *testing.T) {
		protocol := &fakeProtocol{}
		authenticator := newTestAuthenticator(t, protocol, neverCallsBack, io.Discard, time.Second)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := authenticator.Login(ctx, "www.dolthub.com")
		if !errors.Is(err, context.Canceled) || protocol.wasExchanged() {
			t.Fatalf("error = %v, exchanged = %v", err, protocol.wasExchanged())
		}
	})
}

func TestBrowserLoginRejectsUnexpectedPathAndMethod(t *testing.T) {
	for _, tt := range []struct {
		name   string
		method string
		path   string
		want   string
	}{
		{name: "path", method: http.MethodGet, path: "/wrong", want: "unexpected path"},
		{name: "method", method: http.MethodPost, path: "/callback", want: "unsupported method"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			protocol := &fakeProtocol{}
			browser := fakeBrowser{browse: func(rawURL string) error {
				u, _ := url.Parse(rawURL)
				callbackURL, _ := url.Parse(u.Query().Get("redirect_uri"))
				callbackURL.Path = tt.path
				callbackURL.RawQuery = "code=" + testCode + "&state=expected-state"
				req, _ := http.NewRequest(tt.method, callbackURL.String(), nil)
				resp, err := http.DefaultClient.Do(req)
				if err == nil {
					resp.Body.Close()
				}
				return err
			}}
			authenticator := newTestAuthenticator(t, protocol, browser, io.Discard, time.Second)
			_, err := authenticator.Login(context.Background(), "www.dolthub.com")
			if err == nil || !strings.Contains(err.Error(), tt.want) || protocol.wasExchanged() {
				t.Fatalf("error = %v, exchanged = %v", err, protocol.wasExchanged())
			}
		})
	}
}

func TestBrowserLoginPropagatesExchangeAndIdentityFailures(t *testing.T) {
	for _, tt := range []struct {
		name        string
		exchangeErr error
		identityErr error
	}{
		{name: "exchange", exchangeErr: errors.New("exchange failed")},
		{name: "identity", identityErr: errors.New("identity failed")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			protocol := &fakeProtocol{token: credentials.OAuthToken{AccessToken: "token"}, username: "alice", exchangeErr: tt.exchangeErr, identityErr: tt.identityErr}
			authenticator := newTestAuthenticator(t, protocol, callbackBrowser(t, "code="+testCode+"&state=expected-state"), io.Discard, time.Second)
			if _, err := authenticator.Login(context.Background(), "www.dolthub.com"); err == nil {
				t.Fatal("expected login failure")
			}
		})
	}
}

type closeTrackingListener struct {
	net.Listener
	closed atomic.Bool
}

func (l *closeTrackingListener) Close() error {
	l.closed.Store(true)
	return l.Listener.Close()
}

func TestBrowserLoginAlwaysClosesListener(t *testing.T) {
	protocol := &fakeProtocol{}
	var listener *closeTrackingListener
	authenticator, err := NewBrowserAuthenticator(BrowserOptions{
		Protocol:    func(string) (OAuthProtocol, error) { return protocol, nil },
		Browser:     fakeBrowser{browse: func(string) error { return nil }},
		CallbackURL: "http://localhost:0/callback",
		Timeout:     10 * time.Millisecond,
		Listen: func(network, address string) (net.Listener, error) {
			inner, err := net.Listen(network, address)
			if err == nil {
				listener = &closeTrackingListener{Listener: inner}
			}
			return listener, err
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = authenticator.Login(context.Background(), "www.dolthub.com")
	if listener == nil || !listener.closed.Load() {
		t.Fatal("callback listener was not closed")
	}
}

func TestBrowserAuthenticatorRejectsUnsafeCallbackURLs(t *testing.T) {
	protocol := func(string) (OAuthProtocol, error) { return &fakeProtocol{}, nil }
	browser := fakeBrowser{browse: func(string) error { return nil }}
	for _, callbackURL := range []string{"https://localhost:53682/callback", "http://example.com:53682/callback", "http://localhost:53682/other", "http://localhost/callback"} {
		if _, err := NewBrowserAuthenticator(BrowserOptions{Protocol: protocol, Browser: browser, CallbackURL: callbackURL}); err == nil {
			t.Fatalf("callback URL %q was accepted", callbackURL)
		}
	}
}
