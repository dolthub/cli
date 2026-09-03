package dolthub

import (
	"context"
	"net/http"
	"net/url"
)

func (c *Client) ListReleases(ctx context.Context, owner, database, token string) ([]Release, string, error) {
	var out []Release
	q := url.Values{}
	if token != "" {
		q.Set("page_token", token)
	}
	m, e := c.request(ctx, http.MethodGet, path("databases", owner, database, "releases"), q, nil, &out)
	return out, m.NextPageToken, e
}
