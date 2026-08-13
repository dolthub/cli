package authflow

import (
	"context"
	"errors"
)

// LoginResult is a validated browser-login result.
type LoginResult struct{ Host, Username, Token string }

// Authenticator completes browser authentication and validates the resulting identity.
type Authenticator interface {
	Login(context.Context, string) (LoginResult, error)
}

// Unavailable is used until the registered dh OAuth client contract is configured.
type Unavailable struct{}

func (Unavailable) Login(context.Context, string) (LoginResult, error) {
	return LoginResult{}, errors.New("browser login is not configured: the dh OAuth client registration is pending")
}
