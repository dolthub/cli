package credentials

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const DefaultRefreshSkew = time.Minute

// RefreshFunc exchanges a refresh token for a rotated OAuth credential.
// Implementations must not include token values in returned errors.
type RefreshFunc func(context.Context, string) (OAuthToken, error)

// TokenSource loads an account credential and refreshes it at most once at a
// time. A rotated credential is persisted before its access token is returned
// to a caller, preventing use of a token whose refresh-token successor was
// lost locally.
type TokenSource struct {
	Store   Store
	Host    string
	User    string
	Refresh RefreshFunc
	Now     func() time.Time
	Skew    time.Duration

	mu sync.Mutex
}

func (s *TokenSource) AccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Store == nil {
		return "", errors.New("credential store is not configured")
	}
	stored, err := GetStoredOAuthToken(s.Store, s.Host, s.User)
	if err != nil {
		return "", err
	}
	token := stored.Token
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	skew := s.Skew
	if skew == 0 {
		skew = DefaultRefreshSkew
	}
	if !token.NeedsRefresh(now, skew) {
		return token.AccessToken, nil
	}
	if token.RefreshToken == "" {
		return "", errors.New("oauth access token expired and no refresh token is available")
	}
	if s.Refresh == nil {
		return "", errors.New("oauth access token requires refresh, but refresh is not configured")
	}
	rotated, err := s.Refresh(ctx, token.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("refresh oauth credential: %w", err)
	}
	if err := rotated.Validate(); err != nil {
		return "", fmt.Errorf("refresh oauth credential: %w", err)
	}
	if rotated.RefreshToken == "" {
		return "", errors.New("refresh oauth credential: response has no rotated refresh token")
	}
	if err := SetOAuthTokenAt(s.Store, stored.Source, s.Host, s.User, rotated); err != nil {
		return "", fmt.Errorf("persist refreshed oauth credential: %w", err)
	}
	return rotated.AccessToken, nil
}
