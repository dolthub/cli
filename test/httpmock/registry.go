package httpmock

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
)

type response struct {
	status int
	header http.Header
	body   []byte
}
type expectation struct {
	method, path string
	response     response
	called       bool
}

// Registry is a strict RoundTripper whose expected requests must each occur once.
type Registry struct {
	t            testing.TB
	mu           sync.Mutex
	expectations []*expectation
}

func New(t testing.TB) *Registry { t.Helper(); r := &Registry{t: t}; t.Cleanup(r.Verify); return r }
func (r *Registry) Register(method, path string, status int, body string) {
	r.t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expectations = append(r.expectations, &expectation{method: method, path: path, response: response{status: status, header: http.Header{"Content-Type": []string{"application/json"}}, body: []byte(body)}})
}
func (r *Registry) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.expectations {
		if !e.called && e.method == req.Method && e.path == req.URL.EscapedPath() {
			e.called = true
			return &http.Response{StatusCode: e.response.status, Header: e.response.header.Clone(), Body: io.NopCloser(bytes.NewReader(e.response.body)), Request: req}, nil
		}
	}
	return nil, fmt.Errorf("unexpected HTTP request: %s %s", req.Method, req.URL.EscapedPath())
}
func (r *Registry) Verify() {
	r.t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.expectations {
		if !e.called {
			r.t.Errorf("expected HTTP request was not made: %s %s", e.method, e.path)
		}
	}
}
