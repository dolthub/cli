package dolthub

import (
	"context"
	"net/http"
	"net/url"
)

// GetOperation returns one asynchronous operation. Operation IDs are opaque
// and are escaped as a single path segment.
func (c *Client) GetOperation(ctx context.Context, id string) (Operation, error) {
	var result Operation
	err := c.get(ctx, path("operations", id), &result)
	return result, err
}

// GetOperationURL follows an operation reference after validating that it uses
// the configured DoltHub origin.
func (c *Client) GetOperationURL(ctx context.Context, href string) (Operation, error) {
	var result Operation
	err := c.get(ctx, href, &result)
	return result, err
}

func (c *Client) ListOperations(ctx context.Context, owner, database, token string) ([]Operation, string, error) {
	var result []Operation
	query := url.Values{}
	if token != "" {
		query.Set("page_token", token)
	}
	meta, err := c.request(ctx, http.MethodGet, path("databases", owner, database, "operations"), query, nil, &result)
	return result, meta.NextPageToken, err
}
