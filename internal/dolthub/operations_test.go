package dolthub

import (
	"context"
	"net/http"
	"testing"

	"github.com/dolthub/cli/test/httpmock"
)

func TestGetOperationTreatsIDAsOnePathSegment(t *testing.T) {
	reg := httpmock.New(t)
	reg.Register(http.MethodGet, "/api/v2/operations/owners%2Facme%2Frepos%2Fwidgets%2Fjobs%2F1", 200, `{"data":{"id":"owners/acme/repos/widgets/jobs/1","type":"fork","status":"running","created_at":"2026-09-03T12:00:00Z","cancelable":true}}`)
	operation, err := newTestClient(t, reg).GetOperation(context.Background(), "owners/acme/repos/widgets/jobs/1")
	if err != nil || operation.ID == "" || operation.Status != OperationRunning {
		t.Fatalf("operation = %#v, error = %v", operation, err)
	}
}
