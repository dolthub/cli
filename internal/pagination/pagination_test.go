package pagination

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestCollectFollowsOpaqueTokensAndLimit(t *testing.T) {
	var tokens []string
	pages := map[string]Page[int]{
		"":           {Items: []int{1, 2}, NextPageToken: "opaque/+=="},
		"opaque/+==": {Items: []int{3, 4}, NextPageToken: "unused"},
	}
	items, err := Collect(context.Background(), 3, func(_ context.Context, token string) (Page[int], error) {
		tokens = append(tokens, token)
		return pages[token], nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(items, []int{1, 2, 3}) || !reflect.DeepEqual(tokens, []string{"", "opaque/+=="}) {
		t.Fatalf("items = %v, tokens = %v", items, tokens)
	}
}

func TestCollectStopsWithoutNextToken(t *testing.T) {
	items, err := Collect(context.Background(), 10, func(context.Context, string) (Page[string], error) {
		return Page[string]{Items: []string{"only"}}, nil
	})
	if err != nil || !reflect.DeepEqual(items, []string{"only"}) {
		t.Fatalf("items = %v, error = %v", items, err)
	}
}

func TestCollectDetectsRepeatedToken(t *testing.T) {
	_, err := Collect(context.Background(), 10, func(_ context.Context, token string) (Page[int], error) {
		if token == "" {
			return Page[int]{NextPageToken: "same"}, nil
		}
		return Page[int]{NextPageToken: "same"}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "repeated") {
		t.Fatalf("error = %v", err)
	}
}

func TestCollectHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, err := Collect(ctx, 10, func(context.Context, string) (Page[int], error) {
		calls++
		cancel()
		return Page[int]{Items: []int{1}, NextPageToken: "next"}, nil
	})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("error = %v, calls = %d", err, calls)
	}
}

func TestCollectValidatesInputsAndPropagatesErrors(t *testing.T) {
	if _, err := Collect[int](context.Background(), 0, func(context.Context, string) (Page[int], error) { return Page[int]{}, nil }); err == nil {
		t.Fatal("zero limit unexpectedly succeeded")
	}
	want := errors.New("fetch failed")
	if _, err := Collect(context.Background(), 1, func(context.Context, string) (Page[int], error) { return Page[int]{}, want }); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}
