package dolthub

import (
	"context"
	"net/http"
)

// CreateBranch creates a branch from a branch or commit revision.
func (c *Client) CreateBranch(ctx context.Context, owner, database string, request CreateBranchRequest) (Branch, error) {
	var result Branch
	_, err := c.request(ctx, http.MethodPost, path("databases", owner, database, "branches"), nil, request, &result)
	return result, err
}
