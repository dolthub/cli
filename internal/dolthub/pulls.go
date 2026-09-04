package dolthub

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

func (c *Client) ListPulls(ctx context.Context, owner, database, token string) ([]PullSummary, string, error) {
	var out []PullSummary
	q := url.Values{}
	if token != "" {
		q.Set("page_token", token)
	}
	m, e := c.request(ctx, http.MethodGet, path("databases", owner, database, "pulls"), q, nil, &out)
	return out, m.NextPageToken, e
}

// GetPull returns one pull request by its database-local number.
func (c *Client) GetPull(ctx context.Context, owner, database string, number int64) (Pull, error) {
	var out Pull
	err := c.get(ctx, path("databases", owner, database, "pulls", strconv.FormatInt(number, 10)), &out)
	return out, err
}

// ListPullComments returns the top-level comments for one pull request.
func (c *Client) ListPullComments(ctx context.Context, owner, database string, number int64) ([]PullComment, error) {
	var out []PullComment
	err := c.get(ctx, path("databases", owner, database, "pulls", strconv.FormatInt(number, 10), "comments"), &out)
	return out, err
}

// CreatePullComment creates a top-level pull request comment.
func (c *Client) CreatePullComment(ctx context.Context, owner, database string, number int64, request CreatePullCommentRequest) (PullComment, error) {
	var out PullComment
	_, err := c.request(ctx, http.MethodPost, path("databases", owner, database, "pulls", strconv.FormatInt(number, 10), "comments"), nil, request, &out)
	return out, err
}
