package dolthub

import (
	"context"
	"net/url"
)

func (c *Client) ListBranches(ctx context.Context, owner, database, token string) ([]Branch, string, error) {
	var result []Branch
	query := url.Values{}
	if token != "" {
		query.Set("page_token", token)
	}
	meta, err := c.request(ctx, "GET", path("databases", owner, database, "branches"), query, nil, &result)
	return result, meta.NextPageToken, err
}
