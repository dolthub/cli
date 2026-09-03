package dolthub

import (
	"context"
	"net/http"
	"net/url"
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
