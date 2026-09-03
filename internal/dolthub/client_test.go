package dolthub

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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

func TestRequestEncodesBodyQueryAndMetadata(t *testing.T) {
	transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.EscapedPath() != "/api/v2/databases/acme%2Fwest/widgets" {
			t.Fatalf("request = %s %s", req.Method, req.URL.EscapedPath())
		}
		if got := req.URL.Query().Get("page_token"); got != "opaque/+==" {
			t.Fatalf("page token = %q", got)
		}
		if req.Header.Get("Accept") != "application/json" || req.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("headers = %v", req.Header)
		}
		var body map[string]string
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body["state"] != "closed" {
			t.Fatalf("body = %v, error = %v", body, err)
		}
		return response(req, http.StatusOK, `{"data":{"number":7},"meta":{"next_page_token":"next"}}`), nil
	})
	client := newTestClient(t, transport)
	var result struct {
		Number int `json:"number"`
	}
	meta, err := client.request(context.Background(), http.MethodPatch, path("databases", "acme/west", "widgets"), url.Values{"page_token": {"opaque/+=="}}, map[string]string{"state": "closed"}, &result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Number != 7 || meta.NextPageToken != "next" {
		t.Fatalf("result = %#v, meta = %#v", result, meta)
	}
}

func TestRequestRejectsMissingData(t *testing.T) {
	client := newTestClient(t, roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return response(req, http.StatusOK, `{"meta":{}}`), nil
	}))
	if _, err := client.request(context.Background(), http.MethodGet, "user", nil, nil, &User{}); err == nil || !strings.Contains(err.Error(), "no data field") {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveSameOrigin(t *testing.T) {
	client := newTestClient(t, roundTripperFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("HTTP request should not be made")
		return nil, nil
	}))
	for _, raw := range []string{"https://evil.test/api/v2/operations/1", "//evil.test/path", "https://user@example.test/path", "user#fragment"} {
		if _, err := client.resolveSameOrigin(raw); err == nil {
			t.Errorf("resolveSameOrigin(%q) unexpectedly succeeded", raw)
		}
	}
	u, err := client.resolveSameOrigin("https://EXAMPLE.test/api/v2/operations/a%2Fb")
	if err != nil || u.EscapedPath() != "/api/v2/operations/a%2Fb" {
		t.Fatalf("resolved URL = %v, error = %v", u, err)
	}
}

func TestAPIErrorUsesBodyRequestID(t *testing.T) {
	client := newTestClient(t, roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return response(req, http.StatusNotFound, `{"title":"Not found","status":404,"code":"NOT_FOUND","request_id":"req_body"}`), nil
	}))
	_, err := client.request(context.Background(), http.MethodGet, "missing", nil, nil, &User{})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.RequestID != "req_body" {
		t.Fatalf("error = %#v", err)
	}
}

func TestResponseLimit(t *testing.T) {
	client := newTestClient(t, roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body := io.MultiReader(strings.NewReader(`{"data":"`), io.LimitReader(zeroReader{}, maxSuccessResponse), strings.NewReader(`"}`))
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(body), Request: req}, nil
	}))
	if _, err := client.request(context.Background(), http.MethodGet, "large", nil, nil, nil); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %v", err)
	}
}

func response(req *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	return len(p), nil
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
