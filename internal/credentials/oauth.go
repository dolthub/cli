package credentials

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const oauthCredentialPrefix = "dh.oauth.v1:"

// OAuthToken is the complete credential returned by an OAuth token exchange.
// It is serialized as one opaque secret in the platform credential store.
type OAuthToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

func (t OAuthToken) Validate() error {
	if t.AccessToken == "" {
		return errors.New("oauth credential has no access token")
	}
	if t.TokenType != "" && !strings.EqualFold(t.TokenType, "Bearer") {
		return errors.New("oauth credential has an unsupported token type")
	}
	return nil
}

// NeedsRefresh reports whether the access token is expired or will expire
// within skew. Credentials without an expiry are never refreshed automatically.
func (t OAuthToken) NeedsRefresh(now time.Time, skew time.Duration) bool {
	return !t.ExpiresAt.IsZero() && !t.ExpiresAt.After(now.Add(skew))
}

// EncodeOAuthToken creates the versioned opaque value stored in the keyring.
func EncodeOAuthToken(token OAuthToken) (string, error) {
	if err := token.Validate(); err != nil {
		return "", err
	}
	if token.TokenType == "" {
		token.TokenType = "Bearer"
	}
	b, err := json.Marshal(token)
	if err != nil {
		return "", errors.New("encode oauth credential")
	}
	return oauthCredentialPrefix + string(b), nil
}

// DecodeOAuthToken accepts only versioned OAuth credential bundles.
func DecodeOAuthToken(secret string) (OAuthToken, error) {
	if secret == "" {
		return OAuthToken{}, errors.New("credential is empty")
	}
	if !strings.HasPrefix(secret, oauthCredentialPrefix) {
		return OAuthToken{}, errors.New("credential has an unsupported format")
	}
	var token OAuthToken
	if err := json.Unmarshal([]byte(strings.TrimPrefix(secret, oauthCredentialPrefix)), &token); err != nil {
		return OAuthToken{}, errors.New("decode oauth credential")
	}
	if err := token.Validate(); err != nil {
		return OAuthToken{}, err
	}
	if token.TokenType == "" {
		token.TokenType = "Bearer"
	}
	return token, nil
}

func SetOAuthToken(store Store, host, user string, token OAuthToken) error {
	secret, err := EncodeOAuthToken(token)
	if err != nil {
		return err
	}
	return store.Set(host, user, secret)
}

func GetOAuthToken(store Store, host, user string) (OAuthToken, error) {
	secret, err := store.Get(host, user)
	if err != nil {
		return OAuthToken{}, err
	}
	return DecodeOAuthToken(secret)
}
