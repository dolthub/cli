package dolthub

import (
	"context"
	"net/http"
)

// GetDatabase returns one database repository.
func (c *Client) GetDatabase(ctx context.Context, owner, database string) (Database, error) {
	var result Database
	err := c.get(ctx, path("databases", owner, database), &result)
	return result, err
}

// CreateDatabase creates a database repository.
func (c *Client) CreateDatabase(ctx context.Context, request CreateDatabaseRequest) (Database, error) {
	var result Database
	_, err := c.request(ctx, http.MethodPost, "databases", nil, request, &result)
	return result, err
}

// CreateFork starts an asynchronous database fork operation.
func (c *Client) CreateFork(ctx context.Context, owner, database string, request CreateForkRequest) (OperationRef, error) {
	var result OperationRef
	_, err := c.request(ctx, http.MethodPost, path("databases", owner, database, "forks"), nil, request, &result)
	return result, err
}

// ListForks returns the immediate forks of a database repository.
func (c *Client) ListForks(ctx context.Context, owner, database string) ([]DatabaseRef, error) {
	var result []DatabaseRef
	err := c.get(ctx, path("databases", owner, database, "forks"), &result)
	return result, err
}
