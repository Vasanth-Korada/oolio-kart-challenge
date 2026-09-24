# Oolio Kart Challenge: Food Ordering API

[![CI](https://github.com/Vasanth-Korada/oolio-kart-challenge/actions/workflows/ci.yml/badge.svg?branch=submission-v2)](https://github.com/Vasanth-Korada/oolio-kart-challenge/actions/workflows/ci.yml)

Go backend for Oolio's food-ordering OpenAPI 3.1 spec, with coupon validation over ~313M real codes.

- **Version:** 1.1.0 · see [CHANGELOG.md](CHANGELOG.md)
- **Stack:** Go 1.25 · stdlib `net/http` · Postgres 16 (`pgx`) · Docker
- **Frontend (demo only):** [oolio-kart-challenge-web](https://github.com/Vasanth-Korada/oolio-kart-challenge-web)

## Contents

1. [Features](#features)
2. [Quickstart](#quickstart)
3. [Configuration](#configuration)
4. [API](#api)
5. [Architecture](#architecture)
6. [Coupon Validation](#coupon-validation)
7. [Design Decisions](#design-decisions)
8. [Known Limitations](#known-limitations)
9. [Testing](#testing)
10. [Contributing](#contributing)

---

## Features

- **Product catalog:** list all products, fetch one by id
- **Order placement:** prices computed server-side, client prices never trusted
- **Duplicate items merged:** the same `productId` twice becomes one line item
- **Coupon validation at scale:** 313M codes → 96-byte index, O(log n) lookup
- **5% coupon discount:** documented extension to the base spec
- **API-key auth:** constant-time comparison on `POST /order`
- **Postgres persistence:** versioned migrations, one transaction per order
- **Structured logging:** JSON logs with a request id
- **Health checks:** `/healthz` (liveness), `/readyz` (readiness + storage mode)
- **Docker:** one command, distroless runtime image
- **CI on every push:** gofmt, vet, golangci-lint, build, race-enabled tests
- **Postman collection:** 14 requests, each with assertions

---

## Quickstart

```bash
make docker-up          # Postgres + API on :8080
```

- `POST /order` needs the header `api_key: apitest`
- `coupons/coupons.idx` is committed, so no coupon download is needed to run
- No Docker? `go run ./cmd/server` falls back to in-memory storage (reviewer convenience only)

| Command | What it does |
| --- | --- |
| `make run` | Run the API locally |
| `make test` | Unit + contract tests with `-race` |
| `make integration-test` | Postgres repository tests (needs `DATABASE_URL`) |
| `make lint` | golangci-lint |
| `make postman-test` | Postman collection via newman |
| `make fetch-coupons` | Download the 3 source files (~2.1 GB, resumable) |
| `make build-coupon-index` | Rebuild `coupons.idx` from the source files (~5 min) |
| `make docker-down` | Stop containers and drop the volume |

---

## Configuration

| Env var | Default | Purpose |
| --- | --- | --- |
| `PORT` | `8080` | HTTP port |
| `DATABASE_URL` | `postgres://oolio:oolio@localhost:5432/oolio?sslmode=disable` | Postgres DSN |
| `API_KEY` | `apitest` | Key required on `POST /order` |
| `COUPON_INDEX_PATH` | `coupons/coupons.idx` | Coupon index file |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `CORS_ALLOWED_ORIGIN` | `*` | Allowed browser origin for the web frontend |

Docker Compose credentials are overridable via `deploy/.env.example`.

---

## API

Spec: [`api/openapi.yaml`](api/openapi.yaml)

| Method | Path | Auth | Success | Errors |
| --- | --- | --- | --- | --- |
| `GET` | `/product` | none | `200` list | |
| `GET` | `/product/{id}` | none | `200` product | `400` non-integer id, `404` not found |
| `POST` | `/order` | `api_key` | `200` order | `400` bad JSON, `401` no key, `403` wrong key, `422` validation |
| `GET` | `/healthz` | none | `200` | |
| `GET` | `/readyz` | none | `200` + storage mode | `503` database down |

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
- `couponCode` and `subtotal` extend the base spec; `discounts` and `total` are in it
- Errors use one shape: `{ "code": 422, "type": "validation_error", "message": "..." }`

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
<summary><b>Order flow: dry run</b> (a real request traced through every layer)</summary>

![Order placement: dry run](docs/diagrams/order-dry-run.png)
</details>

**Project layout**

```
cmd/server         wiring: config, DB, migrations, coupon index, routes, graceful shutdown
cmd/buildindex     offline tool: coupon source files → coupons.idx
internal/httpapi   handlers, middleware, error envelope
internal/product   model, service, repositories (Postgres + in-memory)
internal/order     model, service (validation, pricing, coupon), repositories
internal/coupon    Validator, Source, build pipeline, index file format
internal/platform  Postgres pool + migrations, logging, UUIDs
migrations/        SQL schema + seed data
docs/diagrams/     diagrams (PNG + editable .drawio)
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
<summary><b>Coupon build: dry run</b> (13 lines through every stage, byte-exact)</summary>

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

## Design Decisions

| Decision | Why |
| --- | --- |
| Stdlib `net/http`, no framework | Go 1.22 routing covers methods and path params |
| `pgx`, no ORM | Plain parameterized SQL, full control |
| UUID v4 order ids | Not guessable, no id enumeration (IDOR) |
| Server-side pricing | Client prices are never trusted |
| `unit_price` stored per line | Later price changes can't rewrite past orders |
| One transaction per order | Never an order without its items |
| Sentinel errors + `errors.Is` | Status codes mapped by identity, not string matching |
| 422 for validation, 400 for bad JSON | Separates "can't parse" from "business rule failed" |
| In-memory fallback | Reviewer convenience; production should fail fast |
| Coupon index committed | Tiny and deterministic; the image never needs 2.1 GB |

---

## Known Limitations

Found in self-review; planned next.

| Limitation | Planned fix |
| --- | --- |
| Money is `float64` in Go (exact `NUMERIC` in Postgres) | `int64` cents |
| One `SELECT` and one `INSERT` per order item | `WHERE id = ANY($1)` + `pgx.Batch` |
| Huge `quantity` overflows Postgres → `500` | Cap quantity → `422` |
| No `Idempotency-Key`: a retry creates a duplicate order | Idempotency key + unique constraint |
| Flat 5% discount is hardcoded | `DiscountPolicy` interface per coupon |
| Committed index holds valid codes in plain text | Build in CI from a private source |
| Service log lines lack `request_id` | Use the request-scoped logger |
| A panic skips the access log line | Move `Logging` outside `Recover` |
| No request body size limit | `http.MaxBytesReader` |
| Migrations have no lock across replicas | `pg_advisory_lock` |

---

## Testing

| Layer | Files | Covers |
| --- | --- | --- |
| Coupon | `internal/coupon/*_test.go` | Build rules, truncated files, index format, real data (`testify/suite`) |
| Service | `service_test.go` | Validation, pricing, merging, coupons |
| Handler | `*_handler_test.go` | Status codes and error mapping |
| Repository | `*_repository_test.go` | In-memory; Postgres with `-tags integration` |
| Contract | `contract_test.go` | Every response validated against `api/openapi.yaml` |
| End to end | `postman/` | 14 requests incl. all error paths |

- Table-driven cases throughout, run with `-race` in CI
- Real-data check: `HAPPYHRS`, `FIFTYOFF` valid, `SUPER100` invalid

---

## Contributing

- **Branches:** `main` is the original submission; ongoing work goes to `submission-v2`
- **Commits:** [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `ci:`)
- **Before pushing:** `make fmt vet lint test`
- **Releases:** recorded in [CHANGELOG.md](CHANGELOG.md), [Semantic Versioning](https://semver.org/)
