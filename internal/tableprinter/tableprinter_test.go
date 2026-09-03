package tableprinter

import (
	"testing"

	"github.com/dolthub/cli/pkg/iostreams"
)

func TestRenderTTYAndNonTTY(t *testing.T) {
	for _, tty := range []bool{false, true} {
		streams, _, out, _ := iostreams.NewTest()
		streams.SetStdoutTTY(tty)
		table := New(streams, "name", "visibility")
		if err := table.AddRow("widgets", "public"); err != nil {
			t.Fatal(err)
		}
		if err := table.Render(); err != nil {
			t.Fatal(err)
		}
		want := "widgets\tpublic\n"
		if tty {
			want = "NAME     VISIBILITY\nwidgets  public\n"
		}
		if out.String() != want {
			t.Errorf("TTY %v output = %q, want %q", tty, out.String(), want)
		}
	}
}

func TestAddRowValidatesWidth(t *testing.T) {
	streams, _, _, _ := iostreams.NewTest()
	if err := New(streams, "one", "two").AddRow("one"); err == nil {
		t.Fatal("AddRow unexpectedly succeeded")
	}
}
