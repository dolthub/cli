package cmdutil

import (
	"fmt"

	"github.com/spf13/cobra"
)

// FlagError reports invalid command-line input and requests usage output.
type FlagError struct{ Err error }

func (e *FlagError) Error() string { return errorString(e.Err, "invalid command-line input") }
func (e *FlagError) Unwrap() error { return e.Err }

// FlagErrorf constructs a command-line validation error.
func FlagErrorf(format string, args ...any) error {
	return &FlagError{Err: fmt.Errorf(format, args...)}
}

// NoArgs rejects positional arguments as a command-line usage error.
func NoArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return FlagErrorf("%s accepts no arguments", cmd.CommandPath())
}

// ExactArgs returns positional validation that is presented as a usage error.
func ExactArgs(count int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != count {
			return FlagErrorf("%s requires exactly %d argument(s)", cmd.CommandPath(), count)
		}
		return nil
	}
}

// SilentError indicates that an error has already been presented to the user.
type SilentError struct{ Err error }

func (e *SilentError) Error() string { return errorString(e.Err, "command failed") }
func (e *SilentError) Unwrap() error { return e.Err }

// CancelError indicates that the user canceled an operation.
type CancelError struct{ Err error }

func (e *CancelError) Error() string { return errorString(e.Err, "operation canceled") }
func (e *CancelError) Unwrap() error { return e.Err }

// NoResultsError indicates a successful operation with an empty result.
type NoResultsError struct{ Err error }

func (e *NoResultsError) Error() string { return errorString(e.Err, "no results") }
func (e *NoResultsError) Unwrap() error { return e.Err }

// AuthError indicates missing or rejected authentication.
type AuthError struct{ Err error }

func (e *AuthError) Error() string { return errorString(e.Err, "authentication required") }
func (e *AuthError) Unwrap() error { return e.Err }

// ExternalCommandError reports a failed child process and its exit code.
type ExternalCommandError struct {
	Err      error
	ExitCode int
}

func (e *ExternalCommandError) Error() string { return errorString(e.Err, "external command failed") }
func (e *ExternalCommandError) Unwrap() error { return e.Err }

func errorString(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	return err.Error()
}
