package dolthub

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dolthub/cli/test/httpmock"
)

func newTestClient(t *testing.T, transport http.RoundTripper) *Client {
	t.Helper()
	base, _ := url.Parse("https://example.test/api/v2/")
	c, err := NewClient(&http.Client{Transport: transport}, base)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCurrentUser(t *testing.T) {
	reg := httpmock.New(t)
	reg.Register(http.MethodGet, "/api/v2/user", 200, `{"data":{"username":"alice","display_name":"Alice","email_addresses":[{"address":"a@example.com","is_primary":true,"is_verified":true}]}}`)
	user, err := newTestClient(t, reg).CurrentUser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if user.Username != "alice" || len(user.EmailAddresses) != 1 {
		t.Fatalf("user=%#v", user)
	}
}

func TestAPIError(t *testing.T) {
	const token = "fake-secret-token"
	reg := httpmock.New(t)
	reg.Register(http.MethodGet, "/api/v2/user", 401, `{"type":"about:blank","title":"Unauthorized","status":401,"detail":"Bearer fake-secret-token\u001b[31m","code":"UNAUTHENTICATED"}`)
	err := func() error { _, err := newTestClient(t, reg).CurrentUser(context.Background()); return err }()
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err=%T %v", err, err)
	}
	if apiErr.Status != 401 || apiErr.Path != "/api/v2/user" || apiErr.Code != "UNAUTHENTICATED" {
		t.Fatalf("api error=%#v", apiErr)
	}
	if strings.Contains(err.Error(), token) || strings.Contains(err.Error(), "Authorization") {
		t.Fatal("error leaked credentials")
	}
	if strings.ContainsRune(err.Error(), '\x1b') {
		t.Fatal("error retained terminal control sequence")
	}
}

func TestMalformedSuccessResponse(t *testing.T) {
	reg := httpmock.New(t)
	reg.Register(http.MethodGet, "/api/v2/user", 200, `{not-json`)
	_, err := newTestClient(t, reg).CurrentUser(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode DoltHub response") {
		t.Fatalf("err = %v", err)
	}
}

func TestCancellation(t *testing.T) {
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err := newTestClient(t, transport).CurrentUser(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v", err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
