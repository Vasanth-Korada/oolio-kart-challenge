# Authentication and authorization

Added after v1.2.0 (in `[Unreleased]`). Kept deliberately simple: the goal is to
show JWT authn/authz in Go, not a full identity system.

## Flow

```
POST /auth/token {username,password} → UserStore.Authenticate (bcrypt)
  → TokenIssuer.Issue → {"accessToken","tokenType":"Bearer","expiresIn"}
POST /order  Authorization: Bearer <jwt>
  → Authenticate (TokenVerifier.Verify → claims in ctx) → RequireScope("create_order") → handler
```

## Code

- `internal/auth/auth.go`: interfaces `TokenIssuer`, `TokenVerifier`, `UserStore`;
  `Claims` (`Subject`, `Scopes`, `ExpiresAt`, `HasScope`); `ScopeCreateOrder`;
  sentinels `ErrInvalidToken`, `ErrInvalidCredentials`.
- `internal/auth/jwt.go`: `JWTManager` on `github.com/golang-jwt/jwt/v5`. HS256
  only (`WithValidMethods`), requires `iss=oolio-kart`, `aud=oolio-kart-api`,
  `exp`, checks `iat`, 30s leeway, non-empty `sub`. Secret ≥ 32 bytes
  (`ErrWeakSecret`). `scope` is a space-separated string (OAuth style); `jti` is a
  UUID. `RandomSecret()` for the unset-`JWT_SECRET` case.
- `internal/auth/users.go`: `MemoryUserStore`, bcrypt `DefaultCost`; unknown users
  compare against a dummy hash so timing matches a wrong password.
- `internal/httpapi/middleware.go`: `Authenticate(verifier, apiKey, AuthMetrics)`
  and `RequireScope(scope)`. `ClaimsFromContext` in `context.go`.
- `internal/httpapi/auth_handler.go`: `AuthHandler.Token`, 1 KiB body cap,
  `Cache-Control: no-store`.

## Rules

- **`api_key` stays.** It is in the base spec, the Postman collection and the web
  frontend. It maps to subject `api_key` with the `create_order` scope.
- **Bearer wins** when both headers are sent; a bad token is 401 even with a
  good `api_key`.
- **Status codes:** no credentials → 401; bad/expired/tampered/`alg: none` token
  → 401 with `WWW-Authenticate: Bearer error="invalid_token"`; wrong `api_key` →
  403 (unchanged from v1); valid token lacking scope → 403 with
  `error="insufficient_scope"`. Login failures are always the same 401 message.
- Error messages go through `json.Encoder`, which HTML-escapes `<`/`>`; keep them
  out of messages.
- Never log tokens or passwords.

## Tests

`internal/auth/*_test.go` forge tokens with the jwt library (expired, wrong
secret, `alg: none`, HS512, wrong iss/aud, missing exp/sub, tampered payload).
`httpapi/middleware_test.go` covers every header combination and the metric
label per case; removing the scope check makes "valid token without scope" fail.
`contract_test.go` validates `/auth/token` and Bearer orders against the spec.

## Deliberately out of scope

Users table, refresh tokens, revocation (`jti` denylist), login rate limiting,
RS256/JWKS. All listed in known-limitations.md with the planned fix.
