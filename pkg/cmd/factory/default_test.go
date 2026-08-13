package factory

import (
	"testing"

	"github.com/dolthub/cli/pkg/iostreams"
)

func TestNew(t *testing.T) {
	streams, _, _, _ := iostreams.NewTest()
	f := New("1.2.3", streams)

	if got, want := f.AppVersion, "1.2.3"; got != want {
		t.Fatalf("AppVersion = %q, want %q", got, want)
	}
	if f.IO != streams {
		t.Fatal("IO does not contain the supplied streams")
	}
}

func TestAPIBaseURL(t *testing.T) {
	base, err := apiBaseURL("dev.dolthub.com")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := base.String(), "https://dev.dolthub.com/api/v2/"; got != want {
		t.Fatalf("base = %q, want %q", got, want)
	}
	for _, host := range []string{"", "https://example.com", "user@example.com", "example.com/path", "example.com:8443"} {
		if _, err := apiBaseURL(host); err == nil {
			t.Errorf("apiBaseURL(%q) unexpectedly succeeded", host)
		}
	}
}
