package oauth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testClient(t *testing.T, transport http.RoundTripper) *Client {
	t.Helper()
	origin, err := url.Parse("https://dev.dolthub.test")
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(&http.Client{Transport: transport}, origin, "dhoci.v1.test-client")
	if err != nil {
		t.Fatal(err)
	}
	client.now = func() time.Time { return time.Unix(1000, 0).UTC() }
	return client
}

func response(req *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}
}

func TestS256ChallengeRFC7636Vector(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	if got, want := S256Challenge(verifier), "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"; got != want {
		t.Fatalf("challenge = %q, want %q", got, want)
	}
}

func TestNewAuthorization(t *testing.T) {
	client := testClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("unexpected request")
		return nil, nil
	}))
	client.random = strings.NewReader(strings.Repeat("a", 32) + strings.Repeat("b", 32))
	attempt, err := client.NewAuthorization("http://localhost:53682/callback")
	if err != nil {
		t.Fatal(err)
	}
	if attempt.State == "" || len(attempt.CodeVerifier) != 43 {
		t.Fatalf("attempt = %#v", attempt)
	}
	if attempt.URL.Scheme != "https" || attempt.URL.Host != "dev.dolthub.test" || attempt.URL.Path != "/oauth/authorize" {
		t.Fatalf("url = %s", attempt.URL)
	}
	q := attempt.URL.Query()
	want := map[string]string{
		"response_type":         "code",
		"client_id":             "dhoci.v1.test-client",
		"redirect_uri":          "http://localhost:53682/callback",
		"scope":                 DefaultScope,
		"state":                 attempt.State,
		"code_challenge":        S256Challenge(attempt.CodeVerifier),
		"code_challenge_method": "S256",
	}
	for key, value := range want {
		if got := q.Get(key); got != value {
			t.Errorf("%s = %q, want %q", key, got, value)
		}
	}
}

func TestExchangeCodePublicClientRequest(t *testing.T) {
	code := "authorization-secret"
	verifier := strings.Repeat("v", 43)
	client := testClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/api/oauth/access_token" {
			t.Fatalf("request = %s %s", req.Method, req.URL)
		}
		if got := req.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Fatalf("content type = %q", got)
		}
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if req.Form.Get("grant_type") != "authorization_code" || req.Form.Get("client_id") != "dhoci.v1.test-client" || req.Form.Get("code") != code || req.Form.Get("code_verifier") != verifier {
			t.Fatalf("form = %#v", req.Form)
		}
		if req.Form.Get("client_secret") != "" {
			t.Fatal("public client sent a client secret")
		}
		return response(req, http.StatusOK, `{"access_token":"access","refresh_token":"refresh","token_type":"Bearer","expires_in":3600}`), nil
	}))
	token, err := client.ExchangeCode(context.Background(), code, "http://localhost:53682/callback", verifier)
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "access" || token.RefreshToken != "refresh" || !token.ExpiresAt.Equal(time.Unix(4600, 0).UTC()) {
		t.Fatalf("token = %#v", token)
	}
}

func TestRefreshPublicClientRequest(t *testing.T) {
	client := testClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if req.Form.Get("grant_type") != "refresh_token" || req.Form.Get("refresh_token") != "old-refresh" || req.Form.Get("client_secret") != "" {
			t.Fatalf("form = %#v", req.Form)
		}
		return response(req, http.StatusOK, `{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":60}`), nil
	}))
	token, err := client.Refresh(context.Background(), "old-refresh")
	if err != nil {
		t.Fatal(err)
	}
	if token.RefreshToken != "new-refresh" {
		t.Fatalf("token = %#v", token)
	}
}

func TestExchangeErrorDoesNotRetainServerDescriptionOrSecrets(t *testing.T) {
	secret := "authorization-secret"
	client := testClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return response(req, http.StatusBadRequest, `{"error":"invalid_grant","error_description":"echo authorization-secret"}`), nil
	}))
	_, err := client.ExchangeCode(context.Background(), secret, "http://localhost:53682/callback", strings.Repeat("v", 43))
	var exchangeErr *ExchangeError
	if !errors.As(err, &exchangeErr) || exchangeErr.Code != "invalid_grant" {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("authorization code leaked")
	}
}

func TestExchangeSanitizesUntrustedErrorCode(t *testing.T) {
	secret := "refresh-secret"
	client := testClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return response(req, http.StatusBadRequest, `{"error":"echo refresh-secret with spaces"}`), nil
	}))
	_, err := client.Refresh(context.Background(), secret)
	var exchangeErr *ExchangeError
	if !errors.As(err, &exchangeErr) || exchangeErr.Code != "oauth_error" || strings.Contains(err.Error(), secret) {
		t.Fatalf("err = %v", err)
	}
}

func TestExchangeRejectsMalformedOrIncompleteSuccess(t *testing.T) {
	for _, body := range []string{
		`not-json`,
		`{"access_token":"","refresh_token":"refresh","token_type":"Bearer","expires_in":60}`,
		`{"access_token":"access","refresh_token":"","token_type":"Bearer","expires_in":60}`,
		`{"access_token":"access","refresh_token":"refresh","token_type":"Bearer","expires_in":0}`,
	} {
		t.Run(body, func(t *testing.T) {
			client := testClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return response(req, http.StatusOK, body), nil
			}))
			_, err := client.Refresh(context.Background(), "refresh")
			var exchangeErr *ExchangeError
			if !errors.As(err, &exchangeErr) || exchangeErr.Code != "invalid_response" {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestExchangeHonorsCancellationWithoutLeakingCause(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := testClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, req.Context().Err()
	}))
	_, err := client.Refresh(ctx, "refresh-secret")
	if !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "refresh-secret") {
		t.Fatalf("err = %v", err)
	}
}

func TestCurrentUsername(t *testing.T) {
	client := testClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/api/v2/user" || req.Header.Get("Authorization") != "Bearer access-secret" {
			t.Fatalf("request = %s, auth = %q", req.URL, req.Header.Get("Authorization"))
		}
		return response(req, http.StatusOK, `{"data":{"username":"alice"}}`), nil
	}))
	username, err := client.CurrentUsername(context.Background(), "access-secret")
	if err != nil || username != "alice" {
		t.Fatalf("username = %q, err = %v", username, err)
	}
}

func TestNewClientRejectsUnsafeOrigins(t *testing.T) {
	for _, raw := range []string{"http://example.com", "https://user@example.com", "https://example.com/path", "https://example.com?query=1"} {
		origin, _ := url.Parse(raw)
		if _, err := NewClient(http.DefaultClient, origin, "client"); err == nil {
			t.Errorf("origin %q accepted", raw)
		}
	}
}
