package operationwaiter

import (
	"context"
	"errors"
	"github.com/dolthub/cli/internal/dolthub"
	"testing"
	"time"
)

type fakeClient struct {
	operations []dolthub.Operation
	ids, hrefs []string
}

func (f *fakeClient) next() (dolthub.Operation, error) {
	if len(f.operations) == 0 {
		return dolthub.Operation{}, errors.New("unexpected fetch")
	}
	o := f.operations[0]
	f.operations = f.operations[1:]
	return o, nil
}
func (f *fakeClient) GetOperation(_ context.Context, id string) (dolthub.Operation, error) {
	f.ids = append(f.ids, id)
	return f.next()
}
func (f *fakeClient) GetOperationURL(_ context.Context, href string) (dolthub.Operation, error) {
	f.hrefs = append(f.hrefs, href)
	return f.next()
}
func TestWaitFollowsHrefWithBackoff(t *testing.T) {
	c := &fakeClient{operations: []dolthub.Operation{{ID: "1", Status: dolthub.OperationQueued}, {ID: "1", Status: dolthub.OperationRunning}, {ID: "1", Status: dolthub.OperationSucceeded}}}
	var delays []time.Duration
	var statuses []dolthub.OperationStatus
	w := Waiter{Client: c, Interval: time.Second, MaxInterval: 4 * time.Second, Random: func() float64 { return .5 }, Sleep: func(_ context.Context, d time.Duration) error { delays = append(delays, d); return nil }, Observe: func(operation dolthub.Operation) { statuses = append(statuses, operation.Status) }}
	o, err := w.Wait(context.Background(), dolthub.OperationRef{Href: "https://example.test/api/v2/operations/1"})
	if err != nil || o.Status != dolthub.OperationSucceeded || len(c.hrefs) != 3 || len(delays) != 2 || delays[0] != time.Second || delays[1] != 2*time.Second || len(statuses) != 3 || statuses[0] != dolthub.OperationQueued || statuses[1] != dolthub.OperationRunning || statuses[2] != dolthub.OperationSucceeded {
		t.Fatalf("operation=%#v hrefs=%v delays=%v statuses=%v err=%v", o, c.hrefs, delays, statuses, err)
	}
}
func TestWaitFailedReturnsOperation(t *testing.T) {
	c := &fakeClient{operations: []dolthub.Operation{{ID: "1", Status: dolthub.OperationFailed, Error: &dolthub.OperationError{Code: "BAD", Title: "Failed", Detail: "reason"}}}}
	o, err := Waiter{Client: c}.WaitID(context.Background(), "a/b")
	var failed *FailedError
	if !errors.As(err, &failed) || o.ID != "1" || failed.Error() != "Failed: reason (BAD)" || c.ids[0] != "a/b" {
		t.Fatalf("operation=%#v error=%v ids=%v", o, err, c.ids)
	}
}
func TestWaitCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := &fakeClient{operations: []dolthub.Operation{{Status: dolthub.OperationRunning}}}
	w := Waiter{Client: c, Sleep: func(context.Context, time.Duration) error { cancel(); return ctx.Err() }}
	_, err := w.WaitID(ctx, "1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}
func TestWaitRejectsUnknownStatusAndMissingInputs(t *testing.T) {
	c := &fakeClient{operations: []dolthub.Operation{{ID: "1", Status: "mystery"}}}
	if _, err := (Waiter{Client: c}).WaitID(context.Background(), "1"); err == nil {
		t.Fatal("unknown status accepted")
	}
	if _, err := (Waiter{Client: c}).Wait(context.Background(), dolthub.OperationRef{}); err == nil {
		t.Fatal("empty href accepted")
	}
}

func TestWaitErrorsDisplayUUIDAndPreserveRequestID(t *testing.T) {
	const uuid = "716a6b3f-4bd4-432e-b7ae-87bead012a3f"
	const full = "repositoryOwners/dolthub/repositories/people/jobs/" + uuid
	for _, tc := range []struct {
		status  dolthub.OperationStatus
		message string
	}{
		{dolthub.OperationFailed, "job " + uuid + " failed"},
		{"mystery", "job " + uuid + " has unknown status \"mystery\""},
	} {
		c := &fakeClient{operations: []dolthub.Operation{{ID: full, Status: tc.status}}}
		op, err := (Waiter{Client: c}).WaitID(context.Background(), full)
		if err == nil || err.Error() != tc.message || op.ID != full || c.ids[0] != full {
			t.Fatalf("operation=%#v error=%v requests=%v", op, err, c.ids)
		}
	}
}
