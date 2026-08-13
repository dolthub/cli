package version

import (
	"bytes"
	"testing"
)

func TestVersion(t *testing.T) {
	var stdout bytes.Buffer
	cmd := NewCmdVersion("1.2.3")
	cmd.SetOut(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "dh version 1.2.3\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
