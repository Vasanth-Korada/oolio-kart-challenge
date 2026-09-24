package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/auth"
)

// Compile-time check: MemoryUserStore satisfies UserStore.
var _ auth.UserStore = (*auth.MemoryUserStore)(nil)

type UserStoreSuite struct {
	suite.Suite
	store *auth.MemoryUserStore
}

func TestUserStoreSuite(t *testing.T) {
	suite.Run(t, new(UserStoreSuite))
}

func (s *UserStoreSuite) SetupSuite() {
	store, err := auth.NewMemoryUserStore()
	s.Require().NoError(err)
	s.Require().NoError(store.Add("demo", "demo1234", auth.ScopeCreateOrder))
	s.store = store
}

func (s *UserStoreSuite) TestAuthenticate() {
	tests := []struct {
		name     string
		username string
		password string
		wantErr  error
	}{
		{name: "correct password", username: "demo", password: "demo1234"},
		{name: "wrong password", username: "demo", password: "wrong", wantErr: auth.ErrInvalidCredentials},
		{name: "unknown user", username: "ghost", password: "demo1234", wantErr: auth.ErrInvalidCredentials},
		{name: "empty password", username: "demo", password: "", wantErr: auth.ErrInvalidCredentials},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			u, err := s.store.Authenticate(context.Background(), tt.username, tt.password)
			if tt.wantErr != nil {
				s.ErrorIs(err, tt.wantErr)
				s.Empty(u.Username)
				return
			}
			s.Require().NoError(err)
			s.Equal("demo", u.Username)
			s.Equal([]string{auth.ScopeCreateOrder}, u.Scopes)
		})
	}
}

func (s *UserStoreSuite) TestAddRejectsEmptyValues() {
	s.Error(s.store.Add("", "pw"))
	s.Error(s.store.Add("someone", ""))
}
