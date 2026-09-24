# Changelog

All notable changes to this project are documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) · Versioning: [Semantic Versioning](https://semver.org/)

## [Unreleased]

### Added

- **Auth diagrams:** `docs/diagrams/auth.drawio` (HLD, LLD, dry run captured on the real stack) embedded in the README Authentication section
- **Config files per environment:** `config/dev.json`, `stage.json`, `prod.json` loaded with viper; `APP_ENV` picks the file, env vars override it
- **Strict config:** unknown keys and bad durations stop the server; stage and prod require `DATABASE_URL`, `API_KEY`, `JWT_SECRET`, `AUTH_USERNAME`, `AUTH_PASSWORD` from the environment

### Changed

- Order diagrams show `Authenticate` + `RequireScope` instead of `APIKeyAuth`, with the re-captured 401 message and CORS headers
- Order diagrams include the `Metrics` middleware (missing since 1.2.0): HLD chain and `GET /metrics`, an LLD box between RequestID and Recover, and a dry-run step with captured values
- HTTP server timeouts and the shutdown deadline come from the config file instead of code
- The in-memory fallback is dev only; stage and prod exit when Postgres is unreachable
- Compose sets only `APP_ENV` and `DATABASE_URL` and passes the other variables through, so the config file wins; `deploy/.env.example` leaves overrides empty
- Default log level in dev is `debug` (was `info`)

## [1.3.0] - 2026-09-24

### Added

- **JWT auth:** `POST /auth/token` exchanges a username and password for an HS256 access token (`golang-jwt/jwt/v5`, 15 min default)
- **Bearer auth on `POST /order`:** `Authorization: Bearer <token>` with the `create_order` scope; the `api_key` header still works
- **Hardening:** algorithm pinned to HS256 (`alg: none` rejected), `iss`, `aud` and `exp` required, secret of at least 32 bytes, bcrypt passwords, one 401 message for unknown user and wrong password, `Cache-Control: no-store` on tokens
- **Config:** `JWT_SECRET` (random per boot when unset), `JWT_TTL`, `AUTH_USERNAME`, `AUTH_PASSWORD`
- **Metric:** `oolio_auth_attempts_total{method,result}` and an Auth row on the Grafana dashboard
- OpenAPI `bearerAuth` scheme and `/auth/token` path; Postman Auth folder (19 requests, 32 assertions); `make traffic` sends JWT traffic

### Changed

- `APIKeyAuth` replaced by `Authenticate` + `RequireScope`; a 401 now carries `WWW-Authenticate: Bearer`
- `config.Load` returns an error (invalid `JWT_TTL`)
- CORS allows the `Authorization` header
- Middleware and contract tests moved to `testify/suite`

## [1.2.0] - 2026-09-24

### Added

- **Metrics:** Prometheus `/metrics` with HTTP rate, errors and latency (by route pattern), orders placed and rejected, order totals, coupon checks, index size, storage mode, Go runtime and process metrics
- **Grafana dashboard:** `deploy/grafana/dashboards/oolio-kart.json` (overview, HTTP, orders & coupons, Go runtime)
- **Grafana Cloud:** Alloy config and a `cloud` Compose profile that push metrics with credentials from `deploy/.env`
- **Local stack:** `local` Compose profile with Prometheus and Grafana, dashboard preloaded
- `make observability-local`, `make observability-cloud`, `make traffic` (`scripts/loadgen.sh`)
- README Observability section with the live Grafana Cloud dashboard link and a screenshot

### Changed

- `order.NewService` takes an `order.Recorder` (nil for none); `RouterDeps` takes optional `Metrics` and `MetricsHandler`
- `make docker-down` stops the observability containers too

## [1.1.1] - 2026-09-24

### Fixed

- **Order quantity:** a huge quantity overflowed Postgres and returned 500; each line item is now capped at 1000 → 422
- **Duplicate items:** quantities are bounded before merging, so two huge duplicates can't overflow into a negative quantity

### Added

- GoDoc comments on every package and exported identifier
- Postman request for the quantity cap (15 requests)
- Claude Code project skill (`.claude/skills/oolio-kart`): working rules, project references, check and diagram-export scripts

### Changed

- Order service and handler tests moved to `testify/suite`
- Order diagrams show the quantity checks and the re-captured 422

## [1.1.0] - 2026-09-24

Branch `submission-v2`. Minor release: the HTTP API and CLI are unchanged; the coupon index format is internal to the repo.

### Changed

- **Coupon keys:** raw 11-byte keys (length byte + code) replace truncated SHA-256; exact match, no collision risk
- **Coupon index:** format `CPX2`, 96 bytes (was `CPX1`, 136 bytes); a `CPX1` file is rejected on load
- **Coupon package:** split by responsibility (`key`, `index`, `source`, `build`, `format`) behind a `Source` interface
- **Coupon build:** accepts any number of source files (the 8-file cap is gone)
- **Coupon tests:** moved to `testify/suite`, with new cases for repeats, partial and failing sources, empty indexes
- **CI:** runs on `submission-v2`; `actions/checkout` and `actions/setup-go` on v7 (Node 24)
- **CI:** golangci-lint pinned to v2.13.2 via `golangci-lint-action`
- **README:** rewritten, with draw.io diagrams (`docs/diagrams/`) and a known-limitations list

### Fixed

- **Truncated source file:** a line cut off mid-file could be indexed as a code; it is now dropped
- **Lint config:** `.golangci.yml` used the v1 key `disable-all`, now the v2 key `default: none`
- **Lint:** gosec G115 on the key length byte

### Added

- `CHANGELOG.md`
- `CLAUDE.md` with project guidelines

## [1.0.0] - 2026-09-22

Original assignment submission (`main`).

### Added

- Product catalog: `GET /product`, `GET /product/{id}`
- Order placement: `POST /order` with server-side pricing, duplicate-item merging, API-key auth
- Coupon validation: offline index over 313M codes (SHA-256 keys, `CPX1`), O(log n) lookup
- 5% discount on a valid coupon
- Postgres persistence with versioned migrations; in-memory fallback
- Health checks, JSON logging with request ids, CORS
- Docker multi-stage build, Docker Compose, Makefile
- Unit, integration, and OpenAPI contract tests; Postman collection
- CI: gofmt, vet, golangci-lint, build, race-enabled tests

[1.3.0]: https://github.com/Vasanth-Korada/oolio-kart-challenge/releases/tag/v1.3.0
[1.2.0]: https://github.com/Vasanth-Korada/oolio-kart-challenge/releases/tag/v1.2.0
[1.1.1]: https://github.com/Vasanth-Korada/oolio-kart-challenge/releases/tag/v1.1.1
[1.1.0]: https://github.com/Vasanth-Korada/oolio-kart-challenge/releases/tag/v1.1.0
[1.0.0]: https://github.com/Vasanth-Korada/oolio-kart-challenge/releases/tag/v1.0.0
