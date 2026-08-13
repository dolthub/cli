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
