package iostreams

import (
	"bytes"
	"io"
	"os"
)

// IOStreams contains the standard streams and their terminal state.
type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer

	stdinTTYOverride  *bool
	stdoutTTYOverride *bool
	stderrTTYOverride *bool
}

// New returns streams backed by the supplied readers and writers.
func New(in io.Reader, out, errOut io.Writer) *IOStreams {
	return &IOStreams{In: in, Out: out, ErrOut: errOut}
}

// NewTest returns streams backed by independently readable buffers.
func NewTest() (*IOStreams, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return New(in, out, errOut), in, out, errOut
}

// IsStdinTTY reports whether the input stream is a terminal.
func (s *IOStreams) IsStdinTTY() bool {
	return ttyState(s.stdinTTYOverride, s.In)
}

// IsStdoutTTY reports whether the output stream is a terminal.
func (s *IOStreams) IsStdoutTTY() bool {
	return ttyState(s.stdoutTTYOverride, s.Out)
}

// IsStderrTTY reports whether the error stream is a terminal.
func (s *IOStreams) IsStderrTTY() bool {
	return ttyState(s.stderrTTYOverride, s.ErrOut)
}

// SetStdinTTY overrides terminal detection for the input stream.
func (s *IOStreams) SetStdinTTY(value bool) { s.stdinTTYOverride = &value }

// SetStdoutTTY overrides terminal detection for the output stream.
func (s *IOStreams) SetStdoutTTY(value bool) { s.stdoutTTYOverride = &value }

// SetStderrTTY overrides terminal detection for the error stream.
func (s *IOStreams) SetStderrTTY(value bool) { s.stderrTTYOverride = &value }

type statStream interface {
	Stat() (os.FileInfo, error)
}

func ttyState(override *bool, stream any) bool {
	if override != nil {
		return *override
	}

	file, ok := stream.(statStream)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
