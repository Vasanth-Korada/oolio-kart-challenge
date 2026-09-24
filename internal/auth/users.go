package auth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// MemoryUserStore holds users in memory with bcrypt-hashed passwords. It
// implements UserStore and is safe for concurrent use.
type MemoryUserStore struct {
	mu    sync.RWMutex
	users map[string]storedUser
	// dummyHash is compared against when the username is unknown, so a miss
	// costs the same bcrypt time as a wrong password.
	dummyHash []byte
}

type storedUser struct {
	hash   []byte
	scopes []string
}

// NewMemoryUserStore returns an empty store.
func NewMemoryUserStore() (*MemoryUserStore, error) {
	dummy, err := bcrypt.GenerateFromPassword([]byte("timing-equaliser"), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash dummy password: %w", err)
	}
	return &MemoryUserStore{users: map[string]storedUser{}, dummyHash: dummy}, nil
}

// Add stores a user, hashing the password with bcrypt. The plain password is
// not kept.
func (s *MemoryUserStore) Add(username, password string, scopes ...string) error {
	if username == "" || password == "" {
		return errors.New("username and password must not be empty")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[username] = storedUser{hash: hash, scopes: slices.Clone(scopes)}
	return nil
}

// Authenticate returns the user when the password matches, or
// ErrInvalidCredentials for an unknown user or a wrong password.
func (s *MemoryUserStore) Authenticate(_ context.Context, username, password string) (User, error) {
	s.mu.RLock()
	u, ok := s.users[username]
	s.mu.RUnlock()

	hash := u.hash
	if !ok {
		hash = s.dummyHash
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil || !ok {
		return User{}, ErrInvalidCredentials
	}
	return User{Username: username, Scopes: slices.Clone(u.scopes)}, nil
}
