package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/idgen"
)

const (
	// Issuer is the iss claim on every token; Verify rejects any other.
	Issuer = "oolio-kart"
	// Audience is the aud claim on every token; Verify rejects any other.
	Audience = "oolio-kart-api"
	// MinSecretLen is the shortest HS256 secret accepted: 32 bytes, the
	// size of the SHA-256 output, as RFC 7518 section 3.2 requires.
	MinSecretLen = 32
	// clockSkew tolerates small clock differences between servers.
	clockSkew = 30 * time.Second
)

// ErrWeakSecret is returned by NewJWTManager for a secret shorter than
// MinSecretLen.
var ErrWeakSecret = fmt.Errorf("jwt secret must be at least %d bytes", MinSecretLen)

// tokenClaims is the JWT payload: the registered claims plus scope, a
// space-separated list as in OAuth 2.0.
type tokenClaims struct {
	Scope string `json:"scope,omitempty"`
	jwt.RegisteredClaims
}

// JWTManager issues and verifies HS256 tokens. It implements TokenIssuer and
// TokenVerifier and is safe for concurrent use.
type JWTManager struct {
	secret []byte
	ttl    time.Duration
}

// NewJWTManager returns a manager signing with secret; tokens expire after
// ttl.
func NewJWTManager(secret []byte, ttl time.Duration) (*JWTManager, error) {
	if len(secret) < MinSecretLen {
		return nil, ErrWeakSecret
	}
	if ttl <= 0 {
		return nil, errors.New("jwt ttl must be positive")
	}
	return &JWTManager{secret: secret, ttl: ttl}, nil
}

// RandomSecret returns MinSecretLen random bytes, for when no secret is
// configured. Tokens signed with it stop verifying once the process exits.
func RandomSecret() ([]byte, error) {
	b := make([]byte, MinSecretLen)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate jwt secret: %w", err)
	}
	return b, nil
}

// Issue signs a token for subject with the given scopes. The jti is a random
// UUID, ready for a future revocation list.
func (m *JWTManager) Issue(subject string, scopes []string) (Token, error) {
	now := time.Now()
	claims := tokenClaims{
		Scope: strings.Join(scopes, " "),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			Subject:   subject,
			Audience:  jwt.ClaimStrings{Audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
			ID:        idgen.NewUUID(),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return Token{}, fmt.Errorf("sign token: %w", err)
	}
	return Token{Value: signed, ExpiresIn: m.ttl}, nil
}

// Verify checks the signature, pins the algorithm to HS256 (so "none" and
// algorithm-confusion tokens fail), and requires a matching iss, aud and an
// unexpired exp. Every failure wraps ErrInvalidToken.
func (m *JWTManager) Verify(token string) (Claims, error) {
	var claims tokenClaims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(Issuer),
		jwt.WithAudience(Audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(clockSkew),
	)
	if err != nil {
		return Claims{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if claims.Subject == "" {
		return Claims{}, fmt.Errorf("%w: missing sub", ErrInvalidToken)
	}
	return Claims{
		Subject:   claims.Subject,
		Scopes:    strings.Fields(claims.Scope),
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}
