package iostreams

import (
	"bytes"
	"io"
	"testing"
)

func TestNewTestUsesIndependentBuffers(t *testing.T) {
	streams, in, out, errOut := NewTest()
	in.WriteString("input")
	streams.Out.Write([]byte("output"))
	streams.ErrOut.Write([]byte("error"))

	gotInput, err := io.ReadAll(streams.In)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(gotInput); got != "input" {
		t.Fatalf("input = %q, want input", got)
	}
	if got := out.String(); got != "output" {
		t.Fatalf("output = %q, want output", got)
	}
	if got := errOut.String(); got != "error" {
		t.Fatalf("error output = %q, want error", got)
	}
}

func TestTTYDetectionAndOverrides(t *testing.T) {
	streams := New(&bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})
	if streams.IsStdinTTY() || streams.IsStdoutTTY() || streams.IsStderrTTY() {
		t.Fatal("buffer-backed streams must not be detected as terminals")
	}

	streams.SetStdinTTY(true)
	streams.SetStdoutTTY(true)
	streams.SetStderrTTY(true)
	if !streams.IsStdinTTY() || !streams.IsStdoutTTY() || !streams.IsStderrTTY() {
		t.Fatal("TTY overrides were not applied independently")
	}

	streams.SetStdoutTTY(false)
	if streams.IsStdoutTTY() {
		t.Fatal("stdout TTY override was not updated")
	}
}
