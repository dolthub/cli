package dolthub

import (
	"context"
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
