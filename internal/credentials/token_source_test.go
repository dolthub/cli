package credentials

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokenSourceReturnsFreshAccessToken(t *testing.T) {
	store := NewMemoryStore()
	now := time.Unix(1000, 0)
	if err := SetOAuthToken(store, "example.com", "alice", OAuthToken{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer", ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	source := &TokenSource{Store: store, Host: "example.com", User: "alice", Now: func() time.Time { return now }, Refresh: func(context.Context, string) (OAuthToken, error) {
		t.Fatal("unexpected refresh")
		return OAuthToken{}, nil
	}}
	got, err := source.AccessToken(context.Background())
	if err != nil || got != "access" {
		t.Fatalf("token = %q, err = %v", got, err)
	}
}

func TestTokenSourceRefreshesFileCredentialInPlace(t *testing.T) {
	keyring := NewMemoryStore()
	file := NewFileStore(filepath.Join(t.TempDir(), "credentials.json"))
	store := NewFallbackStore(keyring, file)
	now := time.Unix(1000, 0)
	old := OAuthToken{AccessToken: "old", RefreshToken: "refresh", TokenType: "Bearer", ExpiresAt: now}
	if err := SetOAuthTokenAt(store, SourceFile, "example.com", "alice", old); err != nil {
		t.Fatal(err)
	}
	source := &TokenSource{Store: store, Host: "example.com", User: "alice", Now: func() time.Time { return now }, Refresh: func(context.Context, string) (OAuthToken, error) {
		return OAuthToken{AccessToken: "new", RefreshToken: "rotated", TokenType: "Bearer", ExpiresAt: now.Add(time.Hour)}, nil
	}}
	if got, err := source.AccessToken(context.Background()); err != nil || got != "new" {
		t.Fatalf("token = %q, error = %v", got, err)
	}
	stored, err := GetStoredOAuthToken(store, "example.com", "alice")
	if err != nil || stored.Source != SourceFile || stored.Token.RefreshToken != "rotated" {
		t.Fatalf("stored = %#v, error = %v", stored, err)
	}
	if _, err := keyring.Get("example.com", "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("keyring error = %v", err)
	}
}

func TestTokenSourceRefreshesAndPersistsRotation(t *testing.T) {
	store := NewMemoryStore()
	now := time.Unix(1000, 0)
	if err := SetOAuthToken(store, "example.com", "alice", OAuthToken{AccessToken: "old-access", RefreshToken: "old-refresh", TokenType: "Bearer", ExpiresAt: now}); err != nil {
		t.Fatal(err)
	}
	source := &TokenSource{Store: store, Host: "example.com", User: "alice", Now: func() time.Time { return now }, Refresh: func(_ context.Context, refresh string) (OAuthToken, error) {
		if refresh != "old-refresh" {
			t.Fatal("wrong refresh token")
		}
		return OAuthToken{AccessToken: "new-access", RefreshToken: "new-refresh", TokenType: "Bearer", ExpiresAt: now.Add(time.Hour)}, nil
	}}
	got, err := source.AccessToken(context.Background())
	if err != nil || got != "new-access" {
		t.Fatalf("token = %q, err = %v", got, err)
	}
	stored, err := GetOAuthToken(store, "example.com", "alice")
	if err != nil || stored.RefreshToken != "new-refresh" {
		t.Fatalf("stored = %#v, err = %v", stored, err)
	}
}

func TestTokenSourceSerializesConcurrentRefresh(t *testing.T) {
	store := NewMemoryStore()
	now := time.Unix(1000, 0)
	if err := SetOAuthToken(store, "example.com", "alice", OAuthToken{AccessToken: "old", RefreshToken: "refresh", TokenType: "Bearer", ExpiresAt: now}); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	source := &TokenSource{Store: store, Host: "example.com", User: "alice", Now: func() time.Time { return now }, Refresh: func(context.Context, string) (OAuthToken, error) {
		calls.Add(1)
		return OAuthToken{AccessToken: "new", RefreshToken: "rotated", TokenType: "Bearer", ExpiresAt: now.Add(time.Hour)}, nil
	}}
	var wg sync.WaitGroup
	errCh := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := source.AccessToken(context.Background())
			if err != nil {
				errCh <- err
			} else if got != "new" {
				errCh <- errors.New("wrong access token")
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("refresh calls = %d", calls.Load())
	}
}

func TestTokenSourceDoesNotReturnUnpersistedRotation(t *testing.T) {
	base := NewMemoryStore()
	now := time.Unix(1000, 0)
	if err := SetOAuthToken(base, "example.com", "alice", OAuthToken{AccessToken: "old", RefreshToken: "refresh", TokenType: "Bearer", ExpiresAt: now}); err != nil {
		t.Fatal(err)
	}
	store := &failSetStore{Store: base}
	source := &TokenSource{Store: store, Host: "example.com", User: "alice", Now: func() time.Time { return now }, Refresh: func(context.Context, string) (OAuthToken, error) {
		return OAuthToken{AccessToken: "new", RefreshToken: "rotated", TokenType: "Bearer", ExpiresAt: now.Add(time.Hour)}, nil
	}}
	if got, err := source.AccessToken(context.Background()); err == nil || got != "" {
		t.Fatalf("token = %q, err = %v", got, err)
	}
}

type failSetStore struct{ Store }

func (s *failSetStore) Set(string, string, string) error { return errors.New("store failed") }
