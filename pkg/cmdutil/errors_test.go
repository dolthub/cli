package cmdutil

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

func TestSemanticErrorsUnwrap(t *testing.T) {
	cause := errors.New("cause")
	errs := []error{
		&FlagError{Err: cause},
		&SilentError{Err: cause},
		&CancelError{Err: cause},
		&NoResultsError{Err: cause},
		&AuthError{Err: cause},
		&ExternalCommandError{Err: cause, ExitCode: 9},
	}

	for _, err := range errs {
		if !errors.Is(err, cause) {
			t.Errorf("%T does not unwrap its cause", err)
		}
	}
}

func TestNoArgs(t *testing.T) {
	cmd := &cobra.Command{Use: "example"}
	if err := NoArgs(cmd, nil); err != nil {
		t.Fatalf("NoArgs() with no arguments returned %v", err)
	}

	err := NoArgs(cmd, []string{"unexpected"})
	var flagErr *FlagError
	if !errors.As(err, &flagErr) {
		t.Fatalf("NoArgs() returned %T, want *FlagError", err)
	}
}

func TestFlagErrorf(t *testing.T) {
	err := FlagErrorf("cannot combine %s", "flags")
	var flagErr *FlagError
	if !errors.As(err, &flagErr) {
		t.Fatalf("FlagErrorf() returned %T", err)
	}
	if got, want := err.Error(), "cannot combine flags"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}
