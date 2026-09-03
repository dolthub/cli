package dolthub

import "context"

// GetDatabase returns one database repository.
func (c *Client) GetDatabase(ctx context.Context, owner, database string) (Database, error) {
	var result Database
	err := c.get(ctx, path("databases", owner, database), &result)
	return result, err
}

// ListForks returns the immediate forks of a database repository.
func (c *Client) ListForks(ctx context.Context, owner, database string) ([]DatabaseRef, error) {
	var result []DatabaseRef
	err := c.get(ctx, path("databases", owner, database, "forks"), &result)
	return result, err
}
