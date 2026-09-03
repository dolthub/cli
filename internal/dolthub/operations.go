package dolthub

import "context"

// GetOperation returns one asynchronous operation. Operation IDs are opaque
// and are escaped as a single path segment.
func (c *Client) GetOperation(ctx context.Context, id string) (Operation, error) {
	var result Operation
	err := c.get(ctx, path("operations", id), &result)
	return result, err
}
