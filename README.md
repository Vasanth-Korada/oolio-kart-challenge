# Oolio Kart Challenge: Food Ordering API

[![CI](https://github.com/Vasanth-Korada/oolio-kart-challenge/actions/workflows/ci.yml/badge.svg?branch=submission-v2)](https://github.com/Vasanth-Korada/oolio-kart-challenge/actions/workflows/ci.yml)

Go backend for Oolio's food-ordering OpenAPI 3.1 spec, with coupon validation over ~313M real codes.

- **Version:** 1.4.0 · see [CHANGELOG.md](CHANGELOG.md)
- **Stack:** Go 1.25 · stdlib `net/http` · Postgres 16 (`pgx`) · viper config · Docker
- **Live dashboard:** [Grafana Cloud](https://brownvalley96.grafana.net/goto/sd9ppg) (sign-in to the Grafana stack required)
- **Frontend (demo only):** [oolio-kart-challenge-web](https://github.com/Vasanth-Korada/oolio-kart-challenge-web)

## Contents

1. [Features](#features)
2. [Quickstart](#quickstart)
3. [Configuration](#configuration)
4. [API](#api)
5. [Authentication](#authentication)
6. [Architecture](#architecture)
7. [Coupon Validation](#coupon-validation)
8. [Observability](#observability)
9. [Design Decisions](#design-decisions)
10. [Known Limitations](#known-limitations)
11. [Testing](#testing)
12. [Contributing](#contributing)

---

## Features

- **Product catalog:** list all products, fetch one by id
- **Order placement:** prices computed server-side, client prices never trusted
- **Duplicate items merged:** the same `productId` twice becomes one line item
- **Coupon validation at scale:** 313M codes → 96-byte index, O(log n) lookup
- **5% coupon discount:** documented extension to the base spec
- **JWT auth:** `POST /auth/token` issues HS256 tokens; `POST /order` checks the Bearer token and its `create_order` scope
- **API-key auth:** still accepted on `POST /order` (base spec), constant-time comparison
- **Postgres persistence:** versioned migrations, one transaction per order
- **Structured logging:** JSON logs with a request id
- **Health checks:** `/healthz` (liveness), `/readyz` (readiness + storage mode)
- **Metrics:** Prometheus `/metrics` (HTTP, orders, coupons, Go runtime) with a Grafana dashboard
- **Docker:** one command, distroless runtime image
- **CI on every push:** gofmt, vet, golangci-lint, build, race-enabled tests
- **Postman collection:** 19 requests, each with assertions

---

## Quickstart

```bash
make docker-up          # Postgres + API on :8080
```

- `POST /order` needs the header `api_key: apitest`, or a JWT from `POST /auth/token` (user `demo`, password `demo1234`)
- `coupons/coupons.idx` is committed, so no coupon download is needed to run
- No Docker? `go run ./cmd/server` (dev config) falls back to in-memory storage (reviewer convenience only)

| Command | What it does |
| --- | --- |
| `make run` | Run the API locally |
| `make test` | Unit + contract tests with `-race` |
| `make integration-test` | Postgres repository tests (needs `DATABASE_URL`) |
| `make lint` | golangci-lint |
| `make postman-test` | Postman collection via newman |
| `make fetch-coupons` | Download the 3 source files (~2.1 GB, resumable) |
| `make build-coupon-index` | Rebuild `coupons.idx` from the source files (~5 min) |
| `make observability-local` | API + local Prometheus (`:9090`) + Grafana (`:3000`) with the dashboard |
| `make observability-cloud` | API + Grafana Alloy pushing metrics to Grafana Cloud (needs `deploy/.env`) |
| `make traffic` | 2 minutes of mixed traffic, incl. every error path |
| `make docker-down` | Stop all containers and drop the volume |

---

## Configuration

Settings live in [`config/`](config), one JSON file per environment, loaded with [viper](https://github.com/spf13/viper).

```
APP_ENV=dev|stage|prod  →  config/<APP_ENV>.json  →  env var overrides  →  validate  →  start
```

| Setting | `dev.json` | `stage.json` | `prod.json` | Env override |
| --- | --- | --- | --- | --- |
| `server.port` | `8080` | `8080` | `8080` | `PORT` |
| `server.*Timeout` | read header 5s, read 10s, write 10s, idle 60s | same | idle 120s | |
| `server.shutdownTimeout` | `10s` | `15s` | `30s` | |
| `log.level` | `debug` | `info` | `info` | `LOG_LEVEL` |
| `cors.allowedOrigin` | `*` | `https://stage.kart.example.com` | `https://kart.example.com` | `CORS_ALLOWED_ORIGIN` |
| `database.url` | local Postgres | env only | env only | `DATABASE_URL` |
| `database.fallbackToMemory` | `true` | `false` | `false` | |
| `coupon.indexPath` | `coupons/coupons.idx` | same | same | `COUPON_INDEX_PATH` |
| `auth.apiKey` | `apitest` | env only | env only | `API_KEY` |
| `auth.jwtSecret` | empty → random per boot | env only | env only | `JWT_SECRET` |
| `auth.jwtTTL` | `15m` | `15m` | `10m` | `JWT_TTL` |
| `auth.username` / `password` | `demo` / `demo1234` | env only | env only | `AUTH_USERNAME` / `AUTH_PASSWORD` |

- **Choosing the file:** `APP_ENV` (default `dev`); `CONFIG_DIR` moves the folder (default `config`)
- **Precedence:** file, then env vars; an empty env var keeps the file value
- **No secrets in stage/prod files:** startup fails and lists every missing variable (`DATABASE_URL`, `API_KEY`, `JWT_SECRET`, `AUTH_USERNAME`, `AUTH_PASSWORD`)
- **Fail fast outside dev:** only `dev` falls back to in-memory storage when Postgres is down
- **Strict parsing:** an unknown key (a typo) or a bad duration stops the server at boot
- **Docker:** the image ships `config/`; Compose sets `APP_ENV` (default `dev`) and `DATABASE_URL`, and passes the other variables through

```bash
APP_ENV=prod DATABASE_URL=... API_KEY=... JWT_SECRET="$(openssl rand -base64 32)" \
  AUTH_USERNAME=... AUTH_PASSWORD=... go run ./cmd/server
```

Docker Compose values are overridable via `deploy/.env.example`, which also lists the Grafana Cloud variables (`GRAFANA_CLOUD_PROM_URL`, `GRAFANA_CLOUD_PROM_USER`, `GRAFANA_CLOUD_API_TOKEN`).

--- | --- | --- |
| `PORT` | `8080` | HTTP port |
| `DATABASE_URL` | `postgres://oolio:oolio@localhost:5432/oolio?sslmode=disable` | Postgres DSN |
| `API_KEY` | `apitest` | Key required on `POST /order` |
| `COUPON_INDEX_PATH` | `coupons/coupons.idx` | Coupon index file |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `CORS_ALLOWED_ORIGIN` | `*` | Allowed browser origin for the web frontend |
| `JWT_SECRET` | random per boot | HS256 signing secret, at least 32 bytes; unset means tokens die on restart |
| `JWT_TTL` | `15m` | Access token lifetime (Go duration) |
| `AUTH_USERNAME` | `demo` | Demo user for `POST /auth/token` |
| `AUTH_PASSWORD` | `demo1234` | Demo user's password (bcrypt-hashed at startup) |

Docker Compose credentials are overridable via `deploy/.env.example`, which also lists the Grafana Cloud variables (`GRAFANA_CLOUD_PROM_URL`, `GRAFANA_CLOUD_PROM_USER`, `GRAFANA_CLOUD_API_TOKEN`).

---

## API

Spec: [`api/openapi.yaml`](api/openapi.yaml)

| Method | Path | Auth | Success | Errors |
| --- | --- | --- | --- | --- |
| `GET` | `/product` | none | `200` list | |
| `GET` | `/product/{id}` | none | `200` product | `400` non-integer id, `404` not found |
| `POST` | `/auth/token` | none | `200` token | `400` bad JSON, `401` wrong credentials |
| `POST` | `/order` | Bearer JWT or `api_key` | `200` order | `400` bad JSON, `401` no or bad credentials, `403` wrong key or missing scope, `422` validation |
| `GET` | `/healthz` | none | `200` | |
| `GET` | `/readyz` | none | `200` + storage mode | `503` database down |
| `GET` | `/metrics` | none | `200` Prometheus text format | |

**Example: `POST /order`** (real response, second product trimmed)

```jsonc
// request
{ "couponCode": "HAPPYHRS",
  "items": [ { "productId": "1", "quantity": 2 },
             { "productId": "3", "quantity": 1 },
             { "productId": "1", "quantity": 1 } ] }
```

```jsonc
// response 200
{
  "id": "5cb5a242-7973-4830-b2b1-57f847d358b3",
  "items": [ { "productId": "1", "quantity": 3 }, { "productId": "3", "quantity": 1 } ],
  "products": [
    { "id": "1", "name": "Waffle with Berries", "price": 6.5, "category": "Waffle",
      "image": { "thumbnail": "https://picsum.photos/seed/product-1/150/150",
                 "mobile": "https://picsum.photos/seed/product-1/375/250",
                 "tablet": "https://picsum.photos/seed/product-1/600/400",
                 "desktop": "https://picsum.photos/seed/product-1/900/600" } }
  ],
  "couponCode": "HAPPYHRS",
  "subtotal": 27.5,
  "discounts": 1.38,
  "total": 26.12
}
```

- `items` are merged: product `1` sent twice becomes quantity `3`
- `quantity` must be 1 to 1000 per line item, after merging; otherwise `422`
- `couponCode` and `subtotal` extend the base spec; `discounts` and `total` are in it
- Errors use one shape: `{ "code": 422, "type": "validation_error", "message": "..." }`

---

## Authentication

![Authentication: high-level design](docs/diagrams/auth-hld.png)

```bash
TOKEN=$(curl -s -X POST localhost:8080/auth/token \
  -d '{"username":"demo","password":"demo1234"}' | jq -r .accessToken)

curl -X POST localhost:8080/order -H "Authorization: Bearer $TOKEN" \
  -d '{"items":[{"productId":"1","quantity":1}]}'
```

```jsonc
// POST /auth/token → 200
{ "accessToken": "eyJhbGciOiJIUzI1NiIs...", "tokenType": "Bearer", "expiresIn": 900 }

// token payload
{ "sub": "demo", "scope": "create_order", "iss": "oolio-kart", "aud": ["oolio-kart-api"],
  "iat": 1790245907, "exp": 1790246807, "jti": "c06627c2-af0a-4ed7-b077-e429b6cc3712" }
```

| Request to `POST /order` | Result |
| --- | --- |
| Valid Bearer token with `create_order` scope | `200` |
| Correct `api_key` header (base spec) | `200` |
| No credentials | `401` + `WWW-Authenticate: Bearer` |
| Expired, tampered, wrong-issuer or `alg: none` token | `401` + `WWW-Authenticate: Bearer error="invalid_token"` |
| Valid token without the `create_order` scope | `403` |
| Wrong `api_key` | `403` |

- **Library:** [`golang-jwt/jwt/v5`](https://github.com/golang-jwt/jwt), passwords with `golang.org/x/crypto/bcrypt`
- **Authentication vs authorization:** `Authenticate` middleware verifies the caller and puts the claims in the context; `RequireScope("create_order")` decides access
- **Algorithm pinned:** only HS256 is accepted, so `alg: none` and algorithm-confusion tokens fail
- **Claims checked:** signature, `iss`, `aud`, `exp` (required), `iat`, 30s clock-skew leeway
- **No user probing:** unknown user and wrong password get the same `401` and the same bcrypt cost
- **Interface-first:** handlers depend on `auth.TokenIssuer`, `auth.TokenVerifier`, `auth.UserStore`; `JWTManager` and `MemoryUserStore` implement them
- **Bearer wins:** if both headers are sent, only the Bearer token is checked

<details>
<summary><b>Auth flow: low-level design</b> (every branch and status code)</summary>

![Authentication: low-level design](docs/diagrams/auth-lld.png)
</details>

<details>
<summary><b>Auth flow: dry run</b> (real login, Bearer order and every error path)</summary>

![Authentication: dry run](docs/diagrams/auth-dry-run.png)
</details>

---

## Architecture

![Order placement: high-level design](docs/diagrams/order-hld.png)

- **Layered:** handler (HTTP) → service (business rules) → repository (storage)
- **Interface-first:** each layer depends on interfaces, so tests use fakes
- **Package by feature:** `product`, `order`, `coupon` each own their slice

<details>
<summary><b>Order flow: low-level design</b> (every branch and status code)</summary>

![Order placement: low-level design](docs/diagrams/order-lld.png)
</details>

<details>
<summary><b>Order flow: dry run</b> (a real request, step by step)</summary>

![Order placement: dry run](docs/diagrams/order-dry-run.png)
</details>

**Project layout**

```
cmd/server         wiring: config, DB, migrations, coupon index, routes, graceful shutdown
cmd/buildindex     offline tool: coupon source files → coupons.idx
internal/httpapi   handlers, middleware (incl. auth), error envelope
internal/auth      JWT issue/verify, user store (bcrypt), claims and scopes
internal/product   model, service, repositories (Postgres + in-memory)
internal/order     model, service (validation, pricing, coupon), repositories
internal/coupon    Validator, Source, build pipeline, index file format
internal/platform  Postgres pool + migrations, logging, metrics, UUIDs
config/            dev.json, stage.json, prod.json (loaded by internal/config with viper)
migrations/        SQL schema + seed data
docs/diagrams/     diagrams (PNG + editable .drawio)
deploy/            Dockerfile, Compose, Alloy, Prometheus, Grafana dashboard
```

---

## Coupon Validation

**Rule:** a code is valid if it is 8 to 10 characters and appears in at least 2 of the 3 source files.

| File | Compressed | Lines | Unique 8-10 char codes |
| --- | --- | --- | --- |
| couponbase1.gz | 655 MB | 107,260,777 | 107,258,700 |
| couponbase2.gz | 729 MB | 107,260,776 | 107,260,726 |
| couponbase3.gz | 738 MB | 98,566,152 | 98,566,151 |
| **Total** | **~2.1 GB** | **313,087,705** | **8 valid codes → 96-byte index** |

![Coupon validation: high-level design](docs/diagrams/coupon-hld.png)

**How it works**

1. **Offline, once:** `cmd/buildindex` streams each gzip file (never fully in memory)
2. **Filter:** keep lines of 8 to 10 characters
3. **Encode:** each code becomes an 11-byte key (length byte + code, zero-padded)
4. **Sort + dedupe per file:** 3 files in parallel (`errgroup`)
5. **k-way merge:** keep keys present in 2+ files; output is already sorted
6. **Write:** `coupons.idx` = 8-byte header + 11 bytes per code
7. **Runtime:** load at startup, binary search per order (O(log n))

![Coupon package: files and interfaces](docs/diagrams/coupon-package-map.png)

- **`Validator`:** what `order.Service` depends on; `Index` or a fail-closed fallback
- **`Source`:** where codes come from; gzip files today, any input can plug in
- **`io.Writer`:** where the index is written; testable without files

<details>
<summary><b>Coupon build: low-level design</b></summary>

![Coupon build: low-level design](docs/diagrams/coupon-lld.png)
</details>

<details>
<summary><b>Coupon build: dry run</b> (13 lines to a 41-byte index)</summary>

![Coupon build: dry run](docs/diagrams/coupon-dry-run.png)
</details>

**Why this design**

- **Build once, not per boot:** 2.1 GB never changes, so the server only loads 96 bytes
- **Sorted slices, not a map:** a map over 313M codes costs tens of GB; slices cost 11 bytes each
- **Raw keys, not hashes:** codes are at most 10 bytes, so matching is exact with no collision risk
- **Fail closed:** a missing index rejects every coupon; a corrupt index stops the server at boot
- **Tolerates a truncated file:** keeps every complete line, drops the cut-off one
- **Resumable download:** size checked against `Content-Length`, resumes on failure

**Memory**

- **Build:** ~1.2 GB per file (11 bytes × ~107M codes), 3 files in parallel
- **Runtime:** 8 codes × 11 bytes; a million valid codes would still be ~11 MB

---

## Observability

```
Go API /metrics → Grafana Alloy (scrape every 15s) → Grafana Cloud Prometheus → Grafana dashboard
```

**Live dashboard:** [brownvalley96.grafana.net](https://brownvalley96.grafana.net/goto/sd9ppg) (Grafana Cloud; sign-in required)

![Grafana dashboard](docs/images/grafana-dashboard.png)

**Metrics** (all prefixed `oolio_`)

| Metric | Type | Labels |
| --- | --- | --- |
| `http_requests_total` | counter | `method`, `route`, `status` |
| `http_request_duration_seconds` | histogram | `method`, `route` |
| `http_requests_in_flight` | gauge | |
| `orders_placed_total` | counter | `coupon` |
| `orders_rejected_total` | counter | `reason` |
| `orders_total_amount` | histogram (dollars) | |
| `coupon_checks_total` | counter | `result` |
| `coupon_index_codes` | gauge | |
| `auth_attempts_total` | counter | `method` (`password`, `bearer`, `api_key`), `result` |
| `storage_info` | gauge | `mode` |

Plus the standard `go_*` and `process_*` metrics.

- **Route labels are patterns** (`GET /product/{id}`), never raw paths, so series stay bounded
- **Interface-first:** `order.Recorder`, `httpapi.HTTPMetrics` and `httpapi.AuthMetrics`; only `internal/platform/metrics` imports Prometheus
- **Private registry:** each `metrics.New()` is independent, so tests don't share state

**Run it**

- **Locally:** `make observability-local`, then `make traffic`, then open `localhost:3000`
- **Grafana Cloud:** copy `deploy/.env.example` to `deploy/.env`, fill the three `GRAFANA_CLOUD_*` values (stack → Connections → Hosted Prometheus metrics; token scope `metrics:write`), then `make observability-cloud`
- **Dashboard in Grafana Cloud:** Dashboards → New → Import → upload `deploy/grafana/dashboards/oolio-kart.json` → pick your Prometheus data source

---

## Design Decisions

| Decision | Why |
| --- | --- |
| Stdlib `net/http`, no framework | Go 1.22 routing covers methods and path params |
| `pgx`, no ORM | Plain parameterized SQL, full control |
| UUID v4 order ids | Not guessable, no id enumeration (IDOR) |
| JWT (HS256) next to `api_key` | Stateless per-user tokens with scopes; `api_key` kept so base-spec clients don't break |
| HS256, not RS256 | One service signs and verifies, so a shared secret is enough; RS256 + JWKS when other services verify |
| Scope checked in its own middleware | Authentication and authorization stay separate and testable |
| Server-side pricing | Client prices are never trusted |
| `unit_price` stored per line | Later price changes can't rewrite past orders |
| One transaction per order | Never an order without its items |
| Batched order queries | One `SELECT … ANY($1)` + one `unnest` insert: 5 statements for any cart size |
| Quantity capped at 1000 per line | Totals stay inside the DB columns; bad input is `422`, not `500` |
| Sentinel errors + `errors.Is` | Status codes mapped by identity, not string matching |
| 422 for validation, 400 for bad JSON | Separates "can't parse" from "business rule failed" |
| In-memory fallback in dev only | Reviewer convenience; stage and prod fail fast |
| Config files per environment, secrets from env | Settings are reviewed in git; secrets never are |
| Prometheus pull model + Alloy | Standard Go client; Alloy forwards to Grafana Cloud with no app changes |
| Coupon index committed | Tiny and deterministic; the image never needs 2.1 GB |

---

## Known Limitations

Found in self-review; planned next.

| Limitation | Planned fix |
| --- | --- |
| Money is `float64` in Go (exact `NUMERIC` in Postgres) | `int64` cents |
| No `Idempotency-Key`: a retry creates a duplicate order | Idempotency key + unique constraint |
| Flat 5% discount is hardcoded | `DiscountPolicy` interface per coupon |
| Committed index holds valid codes in plain text | Build in CI from a private source |
| Service log lines lack `request_id` | Use the request-scoped logger |
| A panic skips the access log line | Move `Logging` outside `Recover` |
| No request body size limit | `http.MaxBytesReader` |
| Migrations have no lock across replicas | `pg_advisory_lock` |
| `/metrics` is public on the API port | Separate internal port or network policy |
| One demo user from env vars, no `users` table | Postgres `users` table behind `auth.UserStore` |
| No refresh tokens or revocation (tokens live until `exp`) | Refresh token rotation; `jti` denylist |
| No rate limit on `POST /auth/token` | Per-IP and per-user limiter |

---

## Testing

| Layer | Files | Covers |
| --- | --- | --- |
| Coupon | `internal/coupon/*_test.go` | Build rules, truncated files, index format, real data (`testify/suite`) |
| Service | `service_test.go` | Validation, pricing, merging, coupons |
| Auth | `internal/auth/*_test.go` | Issue/verify, expired, forged, `alg: none`, wrong iss/aud, bcrypt login |
| Handler | `*_handler_test.go` | Status codes and error mapping |
| Middleware | `middleware_test.go` | Bearer vs `api_key`, 401 vs 403, scope check, CORS, request id |
| Repository | `*_repository_test.go` | In-memory; Postgres with `-tags integration` |
| Contract | `contract_test.go` | Every response validated against `api/openapi.yaml`, incl. `/auth/token` |
| End to end | `postman/` | 19 requests incl. all error paths and the JWT flow |

- Table-driven cases throughout, run with `-race` in CI
- Real-data check: `HAPPYHRS`, `FIFTYOFF` valid, `SUPER100` invalid

---

## Contributing

- **Branches:** `main` is the original submission; ongoing work goes to `submission-v2`
- **Commits:** [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `ci:`)
- **Before pushing:** `make fmt vet lint test`
- **Releases:** recorded in [CHANGELOG.md](CHANGELOG.md), [Semantic Versioning](https://semver.org/)
