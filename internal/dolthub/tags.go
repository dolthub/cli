package dolthub

import (
	"context"
	"net/http"
)

func (c *Client) CreateTag(ctx context.Context, owner, database string, request CreateTagRequest) (Tag, error) {
	var result Tag
	_, err := c.request(ctx, http.MethodPost, path("databases", owner, database, "tags"), nil, request, &result)
	return result, err
}
