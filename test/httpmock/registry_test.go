package httpmock

import (
	"net/http"
	"testing"
)

func TestRegistryExpectedRequest(t *testing.T) {
	reg := New(t)
	reg.Register(http.MethodGet, "/expected", 200, `{}`)
	req, _ := http.NewRequest(http.MethodGet, "https://example.test/expected", nil)
	resp, err := reg.RoundTrip(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("response=%v err=%v", resp, err)
	}
}

func TestRegistryRejectsUnexpectedRequest(t *testing.T) {
	reg := &Registry{t: t}
	req, _ := http.NewRequest(http.MethodPost, "https://example.test/unexpected", nil)
	if _, err := reg.RoundTrip(req); err == nil {
		t.Fatal("unexpected request accepted")
	}
}
