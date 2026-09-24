# Changelog

All notable changes to this project are documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) · Versioning: [Semantic Versioning](https://semver.org/)

## [Unreleased]

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

[1.1.1]: https://github.com/Vasanth-Korada/oolio-kart-challenge/releases/tag/v1.1.1
[1.1.0]: https://github.com/Vasanth-Korada/oolio-kart-challenge/releases/tag/v1.1.0
[1.0.0]: https://github.com/Vasanth-Korada/oolio-kart-challenge/releases/tag/v1.0.0
