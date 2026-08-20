package browser

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSystemBrowseCommands(t *testing.T) {
	tests := []struct {
		goos string
		want []string
	}{
		{goos: "darwin", want: []string{"open", "https://www.dolthub.com/oauth/authorize?state=safe"}},
		{goos: "linux", want: []string{"xdg-open", "https://www.dolthub.com/oauth/authorize?state=safe"}},
		{goos: "windows", want: []string{"rundll32", "url.dll,FileProtocolHandler", "https://www.dolthub.com/oauth/authorize?state=safe"}},
	}
	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			var got []string
			system := System{goos: tt.goos, start: func(name string, args ...string) error {
				got = append([]string{name}, args...)
				return nil
			}}
			if err := system.Browse(tt.want[len(tt.want)-1]); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("command = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSystemBrowseRejectsUnsafeURL(t *testing.T) {
	called := false
	system := System{goos: "linux", start: func(string, ...string) error { called = true; return nil }}
	for _, rawURL := range []string{"", "relative", "file:///tmp/secret", "javascript:alert(1)"} {
		if err := system.Browse(rawURL); err == nil {
			t.Errorf("Browse(%q) unexpectedly succeeded", rawURL)
		}
	}
	if called {
		t.Fatal("start called for an unsafe URL")
	}
}

func TestSystemBrowseReturnsStartFailure(t *testing.T) {
	system := System{goos: "linux", start: func(string, ...string) error { return errors.New("missing") }}
	if err := system.Browse("https://www.dolthub.com"); err == nil || !strings.Contains(err.Error(), "open browser") {
		t.Fatalf("error = %v", err)
	}
}

func TestSystemBrowseRejectsUnsupportedPlatform(t *testing.T) {
	system := System{goos: "plan9", start: func(string, ...string) error { t.Fatal("start called"); return nil }}
	if err := system.Browse("https://www.dolthub.com"); err == nil || !strings.Contains(err.Error(), "plan9") {
		t.Fatalf("error = %v", err)
	}
}
