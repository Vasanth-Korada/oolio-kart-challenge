# Architecture

## Layers

Handler (HTTP only) → Service (business rules only) → Repository (storage only).
Each layer depends on interfaces defined in the consuming package, so tests use
fakes and storage can swap (Postgres or in-memory). Packages are organised by
feature (`product`, `order`, `coupon`), not by layer.

## Request path

`cmd/server/main.go` → `httpapi.NewRouter`:

- Global middleware, outermost first: `RequestID` → `Metrics` (when set) → `Recover` →
  `Logging` → `CORS`. `Metrics` sits outside `Recover` so recovered panics count as 500.
  `chain()` wraps in reverse, so the first in the list is outermost.
- Routes (Go 1.22 `ServeMux` patterns): `GET /product`, `GET /product/{id}`,
  `POST /order` (wrapped in `APIKeyAuth`), `GET /healthz`, `GET /readyz`, and
  `GET /metrics` when `RouterDeps.MetricsHandler` is set.
- Wrong method on a known path → 405 from `ServeMux` (plain text, not the JSON
  envelope). Unknown path → 404.

Subtlety worth knowing: `Logging` sits inside `Recover`, so a panic unwinds past
`Logging` (no access log line) and `Recover` logs with a logger that has no
`request_id`. It is listed in known limitations.

## Startup (`cmd/server`)

1. `config.Load()` from env; JSON logger via `platform/logging`; `metrics.New()`.
2. `postgres.Connect` (pool + ping). On failure: warn and fall back to in-memory
   repositories (reviewer convenience only; production should fail fast).
   `/readyz` reports `"storage": "postgres"` or `"in-memory (fallback)"`; the
   same value goes to the `oolio_storage_info` metric.
3. `postgres.Migrate` applies embedded `migrations/*.sql` in name order, each in
   its own transaction, recorded in `schema_migrations`.
4. `coupon.LoadIndex(COUPON_INDEX_PATH)`: missing file → `NewUnavailableValidator`
   (rejects all coupons, fail closed); corrupt or wrong format → server exits.
   The loaded code count goes to `oolio_coupon_index_codes` (0 for the fallback).
5. `http.Server` timeouts: ReadHeader 5s, Read 10s, Write 10s, Idle 60s.
   SIGINT/SIGTERM → `Shutdown` with a 10s deadline.

## Configuration

| Env var | Default |
| --- | --- |
| `PORT` | `8080` |
| `DATABASE_URL` | `postgres://oolio:oolio@localhost:5432/oolio?sslmode=disable` |
| `API_KEY` | `apitest` |
| `COUPON_INDEX_PATH` | `coupons/coupons.idx` |
| `LOG_LEVEL` | `info` |
| `CORS_ALLOWED_ORIGIN` | `*` |

The `GRAFANA_CLOUD_*` variables in `deploy/.env` are read by Alloy in Compose,
not by the Go server.

`getEnv` treats an empty value as unset, so `CORS_ALLOWED_ORIGIN=""` does not
disable CORS (it becomes `*`). Don't document otherwise.

## Database

- `products(id TEXT PK, name, price NUMERIC(10,2), category, image_*)`, seeded by
  migration 0001; `List` orders by `id::int`.
- `orders(id UUID PK, coupon_code NULL, subtotal, discount, total NUMERIC(10,2), created_at)`.
- `order_items(order_id FK, product_id FK, quantity INTEGER > 0, unit_price NUMERIC(10,2),
  PK(order_id, product_id))`. `unit_price` is a snapshot at order time.
- Go reads prices as `float64` (`price::float8`); money is rounded with
  `roundMoney` (known limitation: should be integer cents).

## API facts that are easy to get wrong

- The order response field is **`discounts`** (plural, from the base spec); the
  DB column and Go field are `discount`.
- `couponCode` and `subtotal` are extensions to the base spec; `discounts` and
  `total` are in it. `Product.image` (thumbnail/mobile/tablet/desktop, picsum
  placeholders) is in the base spec.
- `GET /product/01` works (id is parsed and normalised); `"01"` as an order
  `productId` does not.
- Error envelope everywhere except 405: `{"code", "type", "message"}`.

## Diagrams

`docs/diagrams/order.drawio` (HLD, LLD, dry run) and `coupon.drawio` (HLD,
package map, LLD, dry run), with PNG exports embedded in the README.
