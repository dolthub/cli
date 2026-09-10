package authflow

import (
	"context"

	"github.com/dolthub/cli/internal/credentials"
)

// LoginResult is a validated browser-login result.
type LoginResult struct {
	Host       string
	Username   string
	Credential credentials.OAuthToken
}

// Authenticator completes browser authentication and validates the resulting identity.
type Authenticator interface {
	Login(context.Context, string) (LoginResult, error)
}
