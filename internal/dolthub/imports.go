package dolthub

import (
	"context"
	"net/http"
)

// CreateImportUpload allocates a multipart upload session.
func (c *Client) CreateImportUpload(ctx context.Context, owner, database string, request CreateImportUploadRequest) (ImportUpload, error) {
	var result ImportUpload
	_, err := c.request(ctx, http.MethodPost, path("databases", owner, database, "imports", "uploads"), nil, request, &result)
	return result, err
}

// CreateImport submits a completed upload for import.
func (c *Client) CreateImport(ctx context.Context, owner, database string, request CreateImportRequest) (OperationRef, error) {
	var result OperationRef
	_, err := c.request(ctx, http.MethodPost, path("databases", owner, database, "imports"), nil, request, &result)
	return result, err
}
