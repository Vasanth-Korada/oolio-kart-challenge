// Package auth issues and verifies JWT access tokens and checks user
// credentials. Callers depend on the TokenIssuer, TokenVerifier and UserStore
// interfaces; JWTManager (HS256, via golang-jwt) and MemoryUserStore (bcrypt)
// are the implementations.
package auth
