package upload

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/dolthub/cli/internal/dolthub"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func useTransport(t *testing.T, f transportFunc) {
	t.Helper()
	old := http.DefaultTransport
	http.DefaultTransport = f
	t.Cleanup(func() { http.DefaultTransport = old })
}
func session(n int) dolthub.ImportUpload {
	s := dolthub.ImportUpload{Token: "token", ContentsKey: "key", HTTPMethod: "PUT", Headers: map[string][]string{"X-Upload": {"yes"}}}
	for i := n; i > 0; i-- {
		s.Parts = append(s.Parts, dolthub.ImportUploadPart{PartNumber: i, URL: "https://storage.example/" + string(rune('0'+i)) + "?secret=hidden"})
	}
	return s
}
func response(code int, etag string) *http.Response {
	return &http.Response{StatusCode: code, Header: http.Header{"Etag": {etag}}, Body: io.NopCloser(strings.NewReader(""))}
}
func TestMultipartOrderingAndRetry(t *testing.T) {
	data := append(bytes.Repeat([]byte{'a'}, int(PartSize)), []byte("tail")...)
	var mu sync.Mutex
	attempts := map[string]int{}
	useTransport(t, func(r *http.Request) (*http.Response, error) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("X-Upload") != "yes" {
			t.Error("unexpected headers", r.Header)
		}
		if r.Method != "PUT" || r.ContentLength != int64(len(b)) {
			t.Error("incorrect request")
		}
		mu.Lock()
		defer mu.Unlock()
		attempts[r.URL.Path]++
		if r.URL.Path == "/1" {
			if !bytes.Equal(b, data[:PartSize]) {
				t.Error("first part bytes")
			}
			if attempts[r.URL.Path] == 1 {
				return response(503, ""), nil
			}
		} else if string(b) != "tail" {
			t.Error("last part bytes")
		}
		return response(200, `"`+r.URL.Path+`"`), nil
	})
	var progress int64
	got, err := File(context.Background(), bytes.NewReader(data), int64(len(data)), session(2), func(n int64) {
		if n <= progress {
			t.Error("progress did not increase")
		}
		progress = n
	})
	if err != nil {
		t.Fatal(err)
	}
	a, b := md5.Sum(data[:PartSize]), md5.Sum(data[PartSize:])
	sum := md5.Sum([]byte(base64.StdEncoding.EncodeToString(a[:]) + base64.StdEncoding.EncodeToString(b[:])))
	if got.MD5 != base64.StdEncoding.EncodeToString(sum[:]) || got.Parts[0].ETag != `"/1"` || got.Parts[1].PartNumber != 2 || progress != int64(len(data)) || attempts["/1"] != 2 {
		t.Fatalf("result %#v, attempts %v", got, attempts)
	}
}
func TestFailuresAndRedaction(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		etag   string
		want   string
	}{{"expired", 403, "", "expired"}, {"missing etag", 200, "", "ETag"}, {"redirect", 307, "", "307"}} {
		t.Run(tc.name, func(t *testing.T) {
			useTransport(t, func(*http.Request) (*http.Response, error) { return response(tc.status, tc.etag), nil })
			_, err := File(context.Background(), strings.NewReader("a"), 1, session(1), nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "hidden") {
				t.Fatalf("error: %v", err)
			}
		})
	}
}
func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	useTransport(t, func(r *http.Request) (*http.Response, error) {
		cancel()
		return nil, errors.New("https://storage.example/?secret=hidden")
	})
	_, err := File(ctx, strings.NewReader("a"), 1, session(1), nil)
	if !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "hidden") {
		t.Fatalf("error: %v", err)
	}
}
func TestRejectInvalidSessionsBeforeTransfer(t *testing.T) {
	useTransport(t, func(*http.Request) (*http.Response, error) {
		t.Error("unexpected transfer")
		return response(500, ""), nil
	})
	for _, mutate := range []func(*dolthub.ImportUpload){
		func(s *dolthub.ImportUpload) { s.Parts[0].PartNumber = 2 },
		func(s *dolthub.ImportUpload) { s.Parts[0].URL = "http://storage.example" },
		func(s *dolthub.ImportUpload) { s.Token = "" },
		func(s *dolthub.ImportUpload) { s.Parts = nil },
	} {
		s := session(1)
		mutate(&s)
		if _, err := File(context.Background(), strings.NewReader("a"), 1, s, nil); err == nil {
			t.Fatal("expected invalid session")
		}
	}
	for _, size := range []int64{0, -1, MaxFileSize + 1} {
		if _, err := NumParts(size); err == nil {
			t.Fatal("expected size error")
		}
	}
	for _, tc := range []struct {
		size  int64
		count int
	}{{1, 1}, {PartSize, 1}, {PartSize + 1, 2}, {MaxFileSize, 205}} {
		n, err := NumParts(tc.size)
		if err != nil || n != tc.count {
			t.Fatalf("parts %d: %d, %v", tc.size, n, err)
		}
	}
}
