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

func TestGetOperationURLValidatesOrigin(t *testing.T) {
	client := newTestClient(t, roundTripperFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("request should not be sent to another origin")
		return nil, nil
	}))
	if _, err := client.GetOperationURL(context.Background(), "https://evil.test/api/v2/operations/1"); err == nil {
		t.Fatal("cross-origin operation href accepted")
	}
}

func TestGetOperationURLFollowsSameOriginHref(t *testing.T) {
	reg := httpmock.New(t)
	reg.Register(http.MethodGet, "/api/v2/operations/a%2Fb", 200, `{"data":{"id":"a/b","type":"merge","status":"succeeded","created_at":"2026-09-04T12:00:00Z","cancelable":false}}`)
	operation, err := newTestClient(t, reg).GetOperationURL(context.Background(), "https://example.test/api/v2/operations/a%2Fb")
	if err != nil || operation.ID != "a/b" {
		t.Fatalf("operation=%#v err=%v", operation, err)
	}
}
