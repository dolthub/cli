package progress

import (
	"fmt"

	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/pkg/iostreams"
)

type Reporter struct {
	streams *iostreams.IOStreams
	id      string
	started bool
}

func New(streams *iostreams.IOStreams, id string) *Reporter {
	return &Reporter{streams: streams, id: id}
}

func (r *Reporter) Start() {
	if r == nil || r.streams == nil {
		return
	}
	r.started = true
	if r.streams.IsStderrTTY() {
		_, _ = fmt.Fprintf(r.streams.ErrOut, "Waiting for operation %s...", r.id)
		return
	}
	_, _ = fmt.Fprintf(r.streams.ErrOut, "Waiting for operation %s...\n", r.id)
}

func (r *Reporter) Observe(operation dolthub.Operation) {
	if r == nil || r.streams == nil || !r.streams.IsStderrTTY() {
		return
	}
	id := operation.ID
	if id == "" {
		id = r.id
	}
	_, _ = fmt.Fprintf(r.streams.ErrOut, "\r\x1b[2KWaiting for operation %s: %s", id, operation.Status)
}

func (r *Reporter) Done() {
	if r != nil && r.streams != nil && r.started && r.streams.IsStderrTTY() {
		_, _ = fmt.Fprintln(r.streams.ErrOut)
	}
}
