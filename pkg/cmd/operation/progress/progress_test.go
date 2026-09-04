package progress

import (
	"github.com/dolthub/cli/internal/dolthub"
	"github.com/dolthub/cli/pkg/iostreams"
	"strings"
	"testing"
)

func TestTTYReporterRewritesStatus(t *testing.T) {
	io, _, _, errOut := iostreams.NewTest()
	io.SetStderrTTY(true)
	r := New(io, "pending")
	r.Start()
	r.Observe(dolthub.Operation{ID: "job/1", Status: dolthub.OperationQueued})
	r.Observe(dolthub.Operation{ID: "job/1", Status: dolthub.OperationRunning})
	r.Done()
	got := errOut.String()
	if !strings.Contains(got, "Waiting for operation pending...") || !strings.Contains(got, "\r\x1b[2KWaiting for operation job/1: queued") || !strings.Contains(got, "\r\x1b[2KWaiting for operation job/1: running\n") {
		t.Fatalf("output=%q", got)
	}
}
func TestNonTTYReporterPrintsOnePlainLine(t *testing.T) {
	io, _, _, errOut := iostreams.NewTest()
	r := New(io, "job/1")
	r.Start()
	r.Observe(dolthub.Operation{ID: "job/1", Status: dolthub.OperationRunning})
	r.Done()
	if got := errOut.String(); got != "Waiting for operation job/1...\n" {
		t.Fatalf("output=%q", got)
	}
}
