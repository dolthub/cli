package dolthub

import (
	"context"
	"net/http"
	"net/url"
)

func (c *Client) ListTags(ctx context.Context, owner, database, token string) ([]Tag, string, error) {
	var result []Tag
	query := url.Values{}
	if token != "" {
		query.Set("page_token", token)
	}
	meta, err := c.request(ctx, http.MethodGet, path("databases", owner, database, "tags"), query, nil, &result)
	return result, meta.NextPageToken, err
}
