package dolthub

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dolthub/cli/test/httpmock"
)

func TestGetPullAndListPullComments(t *testing.T) {
	r := httpmock.New(t)
	r.Register(http.MethodGet, "/api/v2/databases/acme%2Fwest/widgets/pulls/42", 200, `{"data":{"pull_number":42,"title":"Fix","state":"open","from_branch":{"database":{"owner":"a","name":"b"},"branch_name":"work"},"to_branch":{"database":{"owner":"acme","name":"widgets"},"branch_name":"main"},"created_at":"2026-09-04T12:00:00Z","creator":"alice"}}`)
	r.Register(http.MethodGet, "/api/v2/databases/acme%2Fwest/widgets/pulls/42/comments", 200, `{"data":[{"comment_id":"c1","author":"bob","body":"Looks good","created_at":"2026-09-04T13:00:00Z","updated_at":"2026-09-04T13:00:00Z"}]}`)
	c := newTestClient(t, r)
	pull, err := c.GetPull(context.Background(), "acme/west", "widgets", 42)
	if err != nil || pull.PullNumber != 42 || pull.FromBranch.BranchName != "work" {
		t.Fatalf("pull=%#v err=%v", pull, err)
	}
	comments, err := c.ListPullComments(context.Background(), "acme/west", "widgets", 42)
	if err != nil || len(comments) != 1 || comments[0].CommentID != "c1" {
		t.Fatalf("comments=%#v err=%v", comments, err)
	}
}

func TestUpdatePull(t *testing.T) {
	state := PullStateClosed
	client := newTestClient(t, roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.EscapedPath() != "/api/v2/databases/acme%2Fwest/widgets/pulls/42" {
			t.Fatalf("request = %s %s", req.Method, req.URL.EscapedPath())
		}
		var body UpdatePullRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.State == nil || *body.State != PullStateClosed || body.Title != nil || body.Description != nil {
			t.Fatalf("body = %#v", body)
		}
		return response(req, http.StatusOK, `{"data":{"pull_number":42,"title":"Fix","state":"closed","from_branch":{"database":{"owner":"a","name":"b"},"branch_name":"work"},"to_branch":{"database":{"owner":"acme","name":"widgets"},"branch_name":"main"},"created_at":"2026-09-04T12:00:00Z","creator":"alice"}}`), nil
	}))
	pull, err := client.UpdatePull(context.Background(), "acme/west", "widgets", 42, UpdatePullRequest{State: &state})
	if err != nil || pull.State != PullStateClosed {
		t.Fatalf("pull=%#v err=%v", pull, err)
	}
}
