package prompt

import (
	"strings"
	"testing"

	"github.com/dolthub/cli/pkg/iostreams"
)

func TestSystemConfirm(t *testing.T) {
	for _, tt := range []struct {
		name, input        string
		defaultValue, want bool
	}{
		{name: "yes", input: "yes\n", want: true},
		{name: "no", input: "n\n", defaultValue: true, want: false},
		{name: "default yes", input: "\n", defaultValue: true, want: true},
		{name: "default no", input: "\n", want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			streams, _, _, errOut := iostreams.NewTest()
			streams.In = strings.NewReader(tt.input)
			got, err := (System{IO: streams}).Confirm("Continue?", tt.defaultValue)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("Confirm() = %v, want %v", got, tt.want)
			}
			if !strings.Contains(errOut.String(), "Continue?") {
				t.Fatalf("prompt = %q", errOut.String())
			}
		})
	}
}

func TestSystemSelect(t *testing.T) {
	streams, in, _, errOut := iostreams.NewTest()
	in.WriteString("2\n")
	selected, err := (System{IO: streams}).Select("Pick one", []string{"first", "second"})
	if err != nil || selected != 1 {
		t.Fatalf("Select() = %d, %v", selected, err)
	}
	if got := errOut.String(); !strings.Contains(got, "1. first") || !strings.Contains(got, "2. second") {
		t.Fatalf("prompt = %q", got)
	}
}
