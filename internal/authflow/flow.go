package authflow

import (
	"context"
	"errors"

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

// Unavailable is used until the registered dh OAuth client contract is configured.
type Unavailable struct{}

func (Unavailable) Login(context.Context, string) (LoginResult, error) {
	return LoginResult{}, errors.New("browser login is not configured: the dh OAuth client registration is pending")
}
