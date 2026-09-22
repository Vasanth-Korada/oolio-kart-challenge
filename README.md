# Oolio Kart Challenge — Food Ordering API

A Go implementation of Oolio's food-ordering OpenAPI 3.1 spec, built for the
advanced backend challenge. Stdlib `net/http` for routing, Postgres for
storage, and a scale-conscious design for the one business rule the
assignment calls out by name: promo-code validation.

## Table of Contents

1. [Quickstart](#quickstart)
2. [Architecture](#architecture)
3. [Coupon Validation](#coupon-validation)
4. [API Reference](#api-reference)
5. [Design Decisions](#design-decisions)
6. [Scalability](#scalability)
7. [Testing](#testing)
8. [Status](#status)

## Quickstart

```bash
make docker-up   # postgres + backend, using the already-built coupons/coupons.idx
```

The server listens on `:8080`. `POST /order` requires an `api_key: apitest`
header (configurable via `API_KEY`). `coupons/coupons.idx` is committed
(136 bytes), so this is all that's needed to run it — `make fetch-coupons`
and `make build-coupon-index` are only for rebuilding that index yourself
from the original ~2.1GB source files (not committed; see below).

Want the UI too? See [oolio-kart-challenge-web](https://github.com/Vasanth-Korada/oolio-kart-challenge-web)
— point it at this server with `VITE_API_BASE_URL=http://localhost:8080`.

## Architecture

Layered, interface-first: handlers depend on `Service` interfaces, services
depend on `Repository`/`Validator` interfaces, never on concrete storage.

```
cmd/server        - wiring: config, DB pool, migrations, coupon index, routes, graceful shutdown
cmd/buildindex     - offline tool: raw coupon files -> coupons.idx
internal/httpapi   - stdlib net/http handlers, middleware, error envelope
internal/product   - model, Service, Repository (Postgres + in-memory)
internal/order     - model, Service (validation, pricing, coupon check), Repository
internal/coupon    - Validator + the index build/query logic
internal/platform  - postgres pool/migrations, structured logging, Prometheus metrics, id generation
migrations/        - SQL schema + product seed data
```

`product` and `order` are organized by feature (each owns its full vertical
slice), not by technical layer — see [Design Decisions](#design-decisions)
for why.

## Coupon Validation

The centerpiece of this assignment. **Rule:** a code is valid iff it's 8–10
characters long **and** appears in at least 2 of the 3 supplied files
(`couponbase1/2/3.gz`).

**Real numbers**, measured against the actual files (not estimated):

| File | Compressed | Lines | Unique candidates (len 8–10) |
| --- | --- | --- | --- |
| couponbase1.gz | 655MB | 107,260,777 | 107,258,700 |
| couponbase2.gz | 729MB | 107,260,776 | 107,260,726 |
| couponbase3.gz | 738MB | 98,566,152 | 98,566,151 |
| **Total** | **~2.1GB** | **313,087,705** | — |

Result: **8 valid codes** survive the ≥2-file rule, written to a **136-byte**
index. Full build (streaming all ~313M lines, hashing, sorting, merging)
takes ~4.5 minutes on a single core, one-time.

**Why this design, not the obvious one:**

- Loading all three files into memory on every boot means re-paying ~2.1GB of
  decompression for data that never changes. Instead, `cmd/buildindex` runs
  once and produces a tiny artifact the server loads at startup.
- A single `map[key]bitmask` accumulating candidates across all three files
  was the first design — until the real numbers came in. At ~313M candidate
  lines, that map's per-entry overhead would cost tens of GB of RAM. Instead,
  each file's candidates are hashed, sorted, and deduplicated into its own
  compact slice (16 bytes/entry, no map overhead), and a k-way merge finds
  codes present in ≥2 sets. Peak memory is roughly the sum of each file's own
  unique candidate count, not a hashmap-inflated multiple of it.
- Fingerprints are SHA-256 truncated to **128 bits**, not a cheaper 64-bit
  hash. At ~3×10⁸ candidate lines, a 64-bit hash's birthday-bound collision
  probability is roughly 1-in-4000 — an unacceptable risk for something that
  gates a real discount. 128 bits (stdlib `crypto/sha256`, no dependency)
  makes that risk negligible, at a cost paid once, offline.
- The three source files are large enough that plain downloads can silently
  stall or truncate mid-transfer (hit first-hand during development — see
  `scripts/fetch_coupons.sh`). The fetch step verifies each file's size
  against the origin's `Content-Length` and resumes until it matches, rather
  than trusting a tool's exit code alone.
- A truncated/corrupted file doesn't abort the whole index build:
  `internal/coupon.scanFile` logs a warning and continues with whatever was
  read cleanly, so a bad download degrades gracefully instead of crashing.

Verified against the assignment's own examples, using the real data:
`HAPPYHRS` and `FIFTYOFF` are valid, `SUPER100` is not (`internal/coupon/index_real_data_test.go`).

**Alternatives considered and rejected:**

- *Bloom filter instead of an exact index.* Rejected on two counts. First,
  cost: a filter sized for ~107M entries at a 1% false-positive rate needs
  `m = -n·ln(p)/(ln 2)² ≈ 1.03 billion bits (~128MB)` *per file* — roughly
  384MB kept resident for the life of the process, since (unlike an exact
  index) a filter can't be pre-reduced to just the overlapping entries; you
  need the full filter to answer queries about codes you haven't seen yet.
  Second, correctness: with a 1% single-file false-positive rate, roughly
  `3 × p² × (1-p) ≈ 0.03%` of genuinely invalid codes would pass the
  ≥2-file rule purely by filter collision — real discounts granted for
  codes that were never valid. Our index is exact (zero false positives)
  and is 384MB → 136 bytes at rest, because the one-time build discards
  everything except the actual overlap.
- *Reprocessing the 3 files on every server boot.* Rejected — it's
  ~2.1GB of compressed, barely-compressible (near-random) data; paying
  that decompression + scan cost on every deploy for data that never
  changes is wasted work. Build once, load a 136-byte file forever after.
- *64-bit fingerprints.* Rejected once the real scale was known — see
  above. The math (not just intuition) drove the choice of 128 bits.
- *A relational "coupon_codes" table in Postgres, checked with a `WHERE
  code = ANY(...)` per order.* Would work, but turns every order into an
  extra round trip to a table that's static after the one-time import,
  for data that's cheaper to hold as an in-process sorted slice than to
  re-fetch over the network per request.

## API Reference

### `GET /product`

List all products. No authentication required.

**Response `200`:**

```json
[
  { "id": "1", "name": "Waffle with Berries", "price": 6.5, "category": "Waffle" }
]
```

### `GET /product/{id}`

| Status | Condition |
| --- | --- |
| 200 | Product found |
| 400 | `id` is not an integer |
| 404 | Product does not exist |

### `POST /order` 🔒

Requires an `api_key` header.

**Request:**

```json
{
  "items": [
    { "productId": "1", "quantity": 1 },
    { "productId": "9", "quantity": 1 }
  ],
  "couponCode": "HAPPYHRS"
}
```

**Response `200`** (a real captured response, not a mock-up):

```json
{
  "id": "7117484d-c0c2-4d84-84e1-f0a079bfbd1e",
  "items": [
    { "productId": "1", "quantity": 1 },
    { "productId": "9", "quantity": 1 }
  ],
  "products": [
    { "id": "1", "name": "Waffle with Berries", "price": 6.5, "category": "Waffle" },
    { "id": "9", "name": "Oat & Raisin Cookie", "price": 3.5, "category": "Cookie" }
  ],
  "couponCode": "HAPPYHRS",
  "subtotal": 10,
  "discount": 0.5,
  "total": 9.5
}
```

| Status | Condition |
| --- | --- |
| 200 | Order placed |
| 400 | Malformed JSON body |
| 401 | Missing `api_key` header |
| 403 | Wrong `api_key` |
| 422 | Empty items, bad quantity, unknown product, or invalid coupon |

The base `Order` schema has no pricing fields — Oolio's spec never defines a
discount amount for any code, so validation alone can't imply one.
`couponCode`, `subtotal`, `discount`, and `total` are added as a documented
extension: `subtotal` is server-computed from real prices (never trusted
from the client), and a valid coupon takes a flat 5% off
(`internal/order.CouponDiscountRate`) since no other rate is specified
anywhere. `additionalProperties` isn't restricted in the base spec, so these
extra fields don't break conformance — `api/openapi.yaml` documents them
explicitly rather than leaving them undocumented.

### Operational endpoints

Not part of the OpenAPI spec, added as standard production hygiene:

| Endpoint | Purpose |
| --- | --- |
| `GET /healthz` | Liveness — always `200` once the process is serving |
| `GET /readyz` | Readiness — `503` if Postgres is unreachable |
| `GET /metrics` | Prometheus exposition (`http_requests_total`, `http_request_duration_seconds`, labeled by route pattern, not raw path) |

## Design Decisions

- **Stdlib-first, two named exceptions.** No HTTP framework, no ORM — Go
  1.22+'s pattern-based `net/http.ServeMux` handles method and path-param
  routing natively. The two deliberate non-stdlib dependencies are `pgx`
  (Go has no built-in Postgres driver) and `prometheus/client_golang`
  (hand-rolling the Prometheus exposition format would be reinventing an
  established wheel poorly).
- **UUID v4 order IDs via `crypto/rand`** (`internal/platform/idgen`), not
  sequential integers. A sequential id lets anyone enumerate `/order/4`,
  `/order/5`, ... — an IDOR (insecure direct object reference) risk. A
  random 122-bit id makes guessing another order's id computationally
  infeasible.
- **Package-by-feature, not package-by-layer.** `internal/product` and
  `internal/order` each own their full vertical slice (model +
  `Repository` + `Service`), rather than a shared top-level
  `handler/`/`service/`/`repository/` split across all entities. Keeps a
  feature's blast radius to one package and sidesteps the import-cycle
  friction Go's compiler enforces when layers need to reference each other
  bidirectionally.
- **Sentinel errors, checked with `errors.Is`, never string-matched.**
  `order.ErrInvalidCoupon` and friends are package-level `errors.New`
  values; the HTTP layer maps them to status codes by identity, safe
  across wrapping (`fmt.Errorf("%w", ...)`) and refactors.
- **Constant-time API-key comparison** (`crypto/subtle.ConstantTimeCompare`),
  not `==` — a plain string comparison leaks how many leading bytes of a
  guess are correct via response timing.
- **Flat 5% discount for any valid coupon.** The spec never defines a
  per-code rate — see [Coupon Validation](#coupon-validation) and the API
  section above.

## Scalability

| Component | Now | To scale further |
| --- | --- | --- |
| Coupon validation | Pre-built 136-byte index, O(log n) binary search | Shard the offline build by byte-range across goroutines/machines — gzip's multistream boundaries are natural split points |
| Products | Postgres, ~10 rows | A cache (Redis) in front of `product.Repository` if the catalog grows large/read-heavy — the interface doesn't change |
| Orders | Postgres, single instance, one transaction per order | Read replicas for reporting; the write path is already minimal |
| API server | Single stateless process | Horizontal — no in-process state beyond the loaded coupon index, so N replicas behind a load balancer work unmodified |
| Coupon index build | Single core, ~4.5 min for 313M lines | Embarrassingly parallel (see above); the index itself scales with the *overlap* between files, not their size |

## Testing

One test file per layer, table-driven with `t.Run` sub-tests throughout:

| Layer | Where | Covers |
| --- | --- | --- |
| Repository | `*_repository_test.go` (product, order) | In-memory repos directly; `*_integration_test.go` (build tag `integration`) exercise the real Postgres repos against a live DB |
| Service | `internal/{product,order}/service_test.go` | Item validation, pricing, coupon checks, persistence — with fakes for the collaborators |
| Handler | `internal/httpapi/{product,order}_handler_test.go` | Status codes and error mapping in isolation, with a fake service |
| Contract | `internal/httpapi/contract_test.go` | Every endpoint's request/response validated against `api/openapi.yaml` via [kin-openapi](https://github.com/getkin/kin-openapi) — proof of spec conformance, not an assertion of it |
| Coupon | `internal/coupon/*_test.go` | Length boundaries, a synthetic fixture, and the real files' documented examples (skipped if `coupons.idx` isn't built) |

```bash
make test              # everything except Postgres integration tests
make integration-test  # Postgres repos, needs DATABASE_URL (e.g. the docker-compose Postgres)
make lint               # golangci-lint
make vet / make fmt     # go vet / gofmt check
```

CI (`.github/workflows/ci.yml`) runs gofmt, vet,
[golangci-lint](https://golangci-lint.run/) (`.golangci.yml` — errcheck,
staticcheck, unused, gosec, and resource-leak checks; a curated set chosen
for real bug/security signal, not a maximal "every linter on" dump), build,
and `make test` on every push/PR.

## Status

Complete: interface-first product/order/coupon layers (Postgres +
in-memory implementations behind each interface), stdlib HTTP with
api-key auth/structured logging/Prometheus metrics/CORS, Docker Compose,
GitHub Actions CI, and OpenAPI contract tests — all green, verified
against a clean `docker compose up --build`, not just unit tests.

A minimal React frontend lives in a
[separate repository](https://github.com/Vasanth-Korada/oolio-kart-challenge-web)
per the assignment's "feel free to explore" note on the UI.

See the commit history for how the design evolved as real data and real
runs surfaced things a plan alone wouldn't have — e.g. the coupon index
moving from a single map to per-file sorted slices once the real ~313M-line
scale was measured, and the fetch script's retry logic existing because a
download genuinely stalled mid-transfer during development.
