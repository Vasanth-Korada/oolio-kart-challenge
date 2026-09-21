# Oolio Kart Challenge — Food Ordering API

A Go implementation of Oolio's food-ordering OpenAPI 3.1 spec, built for the
advanced backend challenge. Stdlib `net/http` for routing, Postgres for
storage, and a scale-conscious design for the one business rule the
assignment calls out by name: promo-code validation.

## Quickstart

```bash
make fetch-coupons        # downloads the 3 raw coupon files (~2.1GB, not committed)
make build-coupon-index   # builds coupons/coupons.idx (committed, ~136 bytes)
make docker-up            # postgres + backend
```

The server listens on `:8080`. `POST /order` requires an `api_key: apitest`
header (configurable via `API_KEY`).

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

## Coupon validation — the centerpiece

**Rule:** a code is valid iff it's 8–10 characters long **and** appears in at
least 2 of the 3 supplied files (`couponbase1/2/3.gz`).

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

- *Bloom filter instead of an exact index.* Rejected — a Bloom filter's
  false positives would mean occasionally granting a real discount for a
  code that was never actually valid. Fine for a cache warm-up check,
  not for something that gates money.
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

**At 10x or 100x this scale:** the current single-process build (~4.5
minutes for ~313M lines) would become the next bottleneck well before
the 136-byte index itself would. The fix is embarrassingly parallel —
shard each file's scan across goroutines (or machines) by byte range,
since gzip's multistream members are natural split points, then merge
the per-shard sorted slices instead of one linear pass. The index
itself would stay small (it scales with the *overlap* between files,
not their size) unless the valid-coupon pool itself grew by orders of
magnitude, at which point the sorted-slice binary search would move to
a proper on-disk structure (e.g. an LSM-backed KV store) instead of an
in-memory slice.

## API

| Method & path | Notes |
| --- | --- |
| `GET /product` | List all products |
| `GET /product/{id}` | 400 non-numeric id, 404 unknown id |
| `POST /order` | Requires `api_key` header (401 missing, 403 wrong); 422 on empty items, bad quantity, unknown product, or invalid coupon |

The `Order` response schema has no `total`/`discount` field, so a coupon's
effect here is binary — it gates the order, it doesn't compute a discount.
Real prices are still resolved and stored server-side, so adding a
`subtotal`/`discountApplied` field is a small, spec-compatible next step.

## Testing

- Unit tests: coupon length boundaries, a synthetic 3-file fixture with known
  overlaps, and the real files' documented examples
  (`internal/coupon/index_real_data_test.go` — skipped automatically if
  `coupons.idx` isn't built yet)
- **Contract tests** (`internal/httpapi/contract_test.go`): every endpoint's
  request and response is validated against `api/openapi.yaml` itself via
  [kin-openapi](https://github.com/getkin/kin-openapi), not just decoded into
  our own structs — proof of spec conformance, not an assertion of it
- CI (`.github/workflows/ci.yml`) runs gofmt, vet, build, and the full test
  suite on every push/PR

## Status

Backend is feature-complete: interface-first product/order/coupon layers,
Postgres storage, stdlib HTTP with auth/logging/metrics, Docker Compose,
CI, and OpenAPI contract tests. A minimal React frontend lives in a
[separate repository](https://github.com/Vasanth-Korada/oolio-kart-challenge-web)
per the assignment's "feel free to explore" note on the UI — see that repo
for setup. See commit history for how the design evolved as real data and
real runs surfaced things a plan alone wouldn't have.
