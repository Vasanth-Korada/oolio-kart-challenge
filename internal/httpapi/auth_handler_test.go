package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/auth"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/httpapi"
)

// fakeUserStore accepts demo/demo1234, or returns err when set.
type fakeUserStore struct{ err error }

func (f fakeUserStore) Authenticate(_ context.Context, username, password string) (auth.User, error) {
	if f.err != nil {
		return auth.User{}, f.err
	}
	if username == "demo" && password == "demo1234" {
		return auth.User{Username: "demo", Scopes: []string{auth.ScopeCreateOrder}}, nil
	}
	return auth.User{}, auth.ErrInvalidCredentials
}

type failingIssuer struct{}

func (failingIssuer) Issue(string, []string) (auth.Token, error) {
	return auth.Token{}, errors.New("signer down")
}

type AuthHandlerSuite struct {
	suite.Suite
	jwt *auth.JWTManager
}

func TestAuthHandlerSuite(t *testing.T) {
	suite.Run(t, new(AuthHandlerSuite))
}

func (s *AuthHandlerSuite) SetupTest() {
	s.jwt = newTestJWT(s.T())
}

func (s *AuthHandlerSuite) post(h *httpapi.AuthHandler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/auth/token", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.Token(rec, req)
	return rec
}

func (s *AuthHandlerSuite) TestIssuesAVerifiableToken() {
	m := &fakeAuthMetrics{}
	h := &httpapi.AuthHandler{Users: fakeUserStore{}, Tokens: s.jwt, Metrics: m}

	rec := s.post(h, `{"username":"demo","password":"demo1234"}`)

	s.Require().Equal(http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	s.Equal("no-store", rec.Header().Get("Cache-Control"))

	var resp struct {
		AccessToken string `json:"accessToken"`
		TokenType   string `json:"tokenType"`
		ExpiresIn   int    `json:"expiresIn"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &resp))
	s.Equal("Bearer", resp.TokenType)
	s.Equal(int(time.Minute.Seconds()), resp.ExpiresIn)

	claims, err := s.jwt.Verify(resp.AccessToken)
	s.Require().NoError(err)
	s.Equal("demo", claims.Subject)
	s.True(claims.HasScope(auth.ScopeCreateOrder))
	s.Equal([]string{"password:success"}, m.calls)
}

func (s *AuthHandlerSuite) TestErrors() {
	tests := []struct {
		name        string
		users       auth.UserStore
		tokens      auth.TokenIssuer
		body        string
		wantStatus  int
		wantMessage string
	}{
		{name: "malformed json", body: `{nope`, wantStatus: http.StatusBadRequest},
		{name: "body over 1 KiB", body: `{"username":"` + strings.Repeat("a", 2048) + `"}`, wantStatus: http.StatusBadRequest},
		{name: "wrong password", body: `{"username":"demo","password":"x"}`, wantStatus: http.StatusUnauthorized, wantMessage: "invalid username or password"},
		{name: "unknown user gets the same message", body: `{"username":"ghost","password":"demo1234"}`, wantStatus: http.StatusUnauthorized, wantMessage: "invalid username or password"},
		{name: "empty credentials", body: `{}`, wantStatus: http.StatusUnauthorized, wantMessage: "invalid username or password"},
		{name: "store error is still 401", users: fakeUserStore{err: errors.New("db down")}, body: `{"username":"demo","password":"demo1234"}`, wantStatus: http.StatusUnauthorized},
		{name: "signing failure is 500", tokens: failingIssuer{}, body: `{"username":"demo","password":"demo1234"}`, wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			h := &httpapi.AuthHandler{Users: tt.users, Tokens: tt.tokens}
			if h.Users == nil {
				h.Users = fakeUserStore{}
			}
			if h.Tokens == nil {
				h.Tokens = s.jwt
			}

			rec := s.post(h, tt.body)

			s.Equal(tt.wantStatus, rec.Code, "body=%s", rec.Body.String())
			s.NotContains(rec.Body.String(), "accessToken")
			if tt.wantMessage != "" {
				s.Contains(rec.Body.String(), tt.wantMessage)
			}
		})
	}
}
