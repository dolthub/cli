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
	r.Observe(dolthub.Operation{ID: "repositoryOwners/dolthub/repositories/people/jobs/716a6b3f-4bd4-432e-b7ae-87bead012a3f", Status: dolthub.OperationQueued})
	r.Observe(dolthub.Operation{ID: "repositoryOwners/dolthub/repositories/people/jobs/716a6b3f-4bd4-432e-b7ae-87bead012a3f", Status: dolthub.OperationRunning})
	r.Done()
	got := errOut.String()
	if !strings.Contains(got, "Waiting for job pending...") || !strings.Contains(got, "\r\x1b[2KWaiting for job 716a6b3f-4bd4-432e-b7ae-87bead012a3f: queued") || !strings.Contains(got, "\r\x1b[2KWaiting for job 716a6b3f-4bd4-432e-b7ae-87bead012a3f: running\n") {
		t.Fatalf("output=%q", got)
	}
}
func TestNonTTYReporterPrintsOnePlainLine(t *testing.T) {
	io, _, _, errOut := iostreams.NewTest()
	r := New(io, "repositoryOwners/dolthub/repositories/people/jobs/716a6b3f-4bd4-432e-b7ae-87bead012a3f")
	r.Start()
	r.Observe(dolthub.Operation{ID: "repositoryOwners/dolthub/repositories/people/jobs/716a6b3f-4bd4-432e-b7ae-87bead012a3f", Status: dolthub.OperationRunning})
	r.Done()
	if got := errOut.String(); got != "Waiting for job 716a6b3f-4bd4-432e-b7ae-87bead012a3f...\n" {
		t.Fatalf("output=%q", got)
	}
}
