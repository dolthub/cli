package root

import "testing"

func TestNewCmdRootDoesNotRequireExternalState(t *testing.T) {
	cmd := NewCmdRoot("test")

	if cmd.Use != "dh" {
		t.Fatalf("Use = %q, want %q", cmd.Use, "dh")
	}
	if cmd.Commands()[0].Name() != "version" {
		t.Fatalf("first command = %q, want %q", cmd.Commands()[0].Name(), "version")
	}
}
