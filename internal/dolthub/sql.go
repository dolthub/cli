package dolthub

import (
	"context"
	"net/http"
)

// RunSQLRead runs a synchronous, read-only SQL query using a body-encoded request.
func (c *Client) RunSQLRead(ctx context.Context, owner, database string, request SQLReadRequest) (QueryResult, error) {
	var out QueryResult
	_, err := c.request(ctx, http.MethodPost, path("databases", owner, database, "sql"), nil, request, &out)
	return out, err
}

// RunSQLWrite starts an asynchronous SQL write operation.
func (c *Client) RunSQLWrite(ctx context.Context, owner, database string, request SQLWriteRequest) (OperationRef, error) {
	var out OperationRef
	_, err := c.request(ctx, http.MethodPost, path("databases", owner, database, "sql-writes"), nil, request, &out)
	return out, err
}
