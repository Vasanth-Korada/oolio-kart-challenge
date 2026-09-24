package auth

import (
	"context"
	"errors"
	"slices"
	"time"
)

// ScopeCreateOrder allows placing orders. It matches the create_order scope
// the OpenAPI spec puts on POST /order.
const ScopeCreateOrder = "create_order"

// Sentinel errors, compared with errors.Is.
var (
	// ErrInvalidToken means a token is malformed, badly signed, expired or
	// issued for another issuer or audience. Callers must not say which.
	ErrInvalidToken = errors.New("invalid token")
	// ErrInvalidCredentials means the username or password is wrong. It never
	// says which one, so usernames can't be probed.
	ErrInvalidCredentials = errors.New("invalid credentials")
)

// Claims is the verified identity carried by a request.
type Claims struct {
	Subject   string
	Scopes    []string
	ExpiresAt time.Time
}

// HasScope reports whether the claims grant scope.
func (c Claims) HasScope(scope string) bool {
	return slices.Contains(c.Scopes, scope)
}

// Token is a signed access token and how long it stays valid.
type Token struct {
	Value     string
	ExpiresIn time.Duration
}

// User is an authenticated user and the scopes granted to them.
type User struct {
	Username string
	Scopes   []string
}

// TokenIssuer signs access tokens.
type TokenIssuer interface {
	Issue(subject string, scopes []string) (Token, error)
}

// TokenVerifier checks a token and returns its claims, or an error wrapping
// ErrInvalidToken.
type TokenVerifier interface {
	Verify(token string) (Claims, error)
}

// UserStore checks a username and password, returning ErrInvalidCredentials
// when either is wrong.
type UserStore interface {
	Authenticate(ctx context.Context, username, password string) (User, error)
}
