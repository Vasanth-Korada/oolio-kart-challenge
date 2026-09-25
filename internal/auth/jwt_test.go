package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/auth"
)

// Compile-time checks: JWTManager satisfies both token interfaces.
var (
	_ auth.TokenIssuer   = (*auth.JWTManager)(nil)
	_ auth.TokenVerifier = (*auth.JWTManager)(nil)
)

var testSecret = []byte("0123456789abcdef0123456789abcdef")

type JWTSuite struct {
	suite.Suite
	m *auth.JWTManager
}

func TestJWTSuite(t *testing.T) {
	suite.Run(t, new(JWTSuite))
}

func (s *JWTSuite) SetupTest() {
	m, err := auth.NewJWTManager(testSecret, 15*time.Minute)
	s.Require().NoError(err)
	s.m = m
}

// validClaims are claims Verify accepts; cases change one field each.
func validClaims() jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss":   auth.Issuer,
		"aud":   auth.Audience,
		"sub":   "demo",
		"scope": auth.ScopeCreateOrder,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Minute).Unix(),
	}
}

func sign(method jwt.SigningMethod, claims jwt.MapClaims, key any) string {
	signed, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		panic(err)
	}
	return signed
}

func (s *JWTSuite) TestNewJWTManagerValidation() {
	tests := []struct {
		name    string
		secret  []byte
		ttl     time.Duration
		wantErr bool
	}{
		{name: "31-byte secret is too short", secret: testSecret[:31], ttl: time.Minute, wantErr: true},
		{name: "zero ttl", secret: testSecret, ttl: 0, wantErr: true},
		{name: "32-byte secret and positive ttl", secret: testSecret, ttl: time.Minute},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			_, err := auth.NewJWTManager(tt.secret, tt.ttl)
			if tt.wantErr {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *JWTSuite) TestIssueThenVerify() {
	tok, err := s.m.Issue("demo", []string{auth.ScopeCreateOrder, "read"})
	s.Require().NoError(err)
	s.Equal(15*time.Minute, tok.ExpiresIn)
	s.Len(strings.Split(tok.Value, "."), 3, "a JWS compact token has three parts")

	claims, err := s.m.Verify(tok.Value)
	s.Require().NoError(err)
	s.Equal("demo", claims.Subject)
	s.Equal([]string{auth.ScopeCreateOrder, "read"}, claims.Scopes)
	s.True(claims.HasScope(auth.ScopeCreateOrder))
	s.False(claims.HasScope("admin"))
	s.WithinDuration(time.Now().Add(15*time.Minute), claims.ExpiresAt, 2*time.Second)
}

func (s *JWTSuite) TestVerifyRejects() {
	with := func(key string, value any) jwt.MapClaims {
		c := validClaims()
		if value == nil {
			delete(c, key)
		} else {
			c[key] = value
		}
		return c
	}
	tampered := func() string {
		tok, err := s.m.Issue("demo", []string{auth.ScopeCreateOrder})
		s.Require().NoError(err)
		parts := strings.Split(tok.Value, ".")
		// Re-sign nothing: swap the payload for one claiming another user.
		parts[1] = strings.Split(sign(jwt.SigningMethodHS256, with("sub", "admin"), []byte("x")), ".")[1]
		return strings.Join(parts, ".")
	}

	tests := []struct {
		name  string
		token string
	}{
		{name: "empty", token: ""},
		{name: "garbage", token: "not.a.jwt"},
		{name: "wrong secret", token: sign(jwt.SigningMethodHS256, validClaims(), []byte("another-secret-another-secret-xx"))},
		{name: "alg none", token: sign(jwt.SigningMethodNone, validClaims(), jwt.UnsafeAllowNoneSignatureType)},
		{name: "HS512 instead of HS256", token: sign(jwt.SigningMethodHS512, validClaims(), testSecret)},
		{name: "expired", token: sign(jwt.SigningMethodHS256, with("exp", time.Now().Add(-time.Hour).Unix()), testSecret)},
		{name: "missing exp", token: sign(jwt.SigningMethodHS256, with("exp", nil), testSecret)},
		{name: "issued in the future", token: sign(jwt.SigningMethodHS256, with("iat", time.Now().Add(time.Hour).Unix()), testSecret)},
		{name: "wrong issuer", token: sign(jwt.SigningMethodHS256, with("iss", "evil"), testSecret)},
		{name: "wrong audience", token: sign(jwt.SigningMethodHS256, with("aud", "other-api"), testSecret)},
		{name: "missing subject", token: sign(jwt.SigningMethodHS256, with("sub", nil), testSecret)},
		{name: "tampered payload", token: tampered()},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			_, err := s.m.Verify(tt.token)
			s.ErrorIs(err, auth.ErrInvalidToken)
		})
	}
}

func (s *JWTSuite) TestRandomSecretWorksWithManager() {
	secret, err := auth.RandomSecret()
	s.Require().NoError(err)
	s.Len(secret, auth.MinSecretLen)

	other, err := auth.RandomSecret()
	s.Require().NoError(err)
	s.NotEqual(secret, other)

	_, err = auth.NewJWTManager(secret, time.Minute)
	s.NoError(err)
}
