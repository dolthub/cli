// Package pagination collects cursor-paginated API results.
package pagination

import (
	"context"
	"errors"
	"fmt"
)

type Page[T any] struct {
	Items         []T
	NextPageToken string
}

type Fetch[T any] func(context.Context, string) (Page[T], error)

// Collect fetches pages until limit items are collected or the server returns
// no next token. Tokens are treated as opaque values.
func Collect[T any](ctx context.Context, limit int, fetch Fetch[T]) ([]T, error) {
	if limit < 1 {
		return nil, errors.New("pagination limit must be greater than zero")
	}
	if fetch == nil {
		return nil, errors.New("pagination fetch function is required")
	}

	items := make([]T, 0, limit)
	token := ""
	seen := map[string]bool{}
	for len(items) < limit {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		page, err := fetch(ctx, token)
		if err != nil {
			return nil, err
		}
		remaining := limit - len(items)
		if len(page.Items) > remaining {
			page.Items = page.Items[:remaining]
		}
		items = append(items, page.Items...)
		if len(items) == limit || page.NextPageToken == "" {
			return items, nil
		}
		if page.NextPageToken == token || seen[page.NextPageToken] {
			return nil, fmt.Errorf("pagination token %q was repeated", page.NextPageToken)
		}
		seen[page.NextPageToken] = true
		token = page.NextPageToken
	}
	return items, nil
}
