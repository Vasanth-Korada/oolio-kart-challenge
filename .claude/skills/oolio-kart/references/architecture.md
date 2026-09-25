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
  `POST /auth/token`, `POST /order` (wrapped in `Authenticate` + `RequireScope`,
  see [auth.md](auth.md)), `GET /healthz`, `GET /readyz`, and
  `GET /metrics` when `RouterDeps.MetricsHandler` is set.
- Wrong method on a known path → 405 from `ServeMux` (plain text, not the JSON
  envelope). Unknown path → 404.

Subtlety worth knowing: `Logging` sits inside `Recover`, so a panic unwinds past
`Logging` (no access log line) and `Recover` logs with a logger that has no
`request_id`. It is listed in known limitations.

## Startup (`cmd/server`)

1. `config.Load()` (file + env, validated; any error exits); JSON logger via
   `platform/logging`; `metrics.New()`; `newAuth` builds the `JWTManager` (random
   secret + warning when `JWT_SECRET` is unset) and the one-user `MemoryUserStore`.
2. `postgres.Connect` (pool + ping). On failure: dev (`fallbackToMemory`) warns
   and falls back to in-memory repositories; stage and prod exit.
   `/readyz` reports `"storage": "postgres"` or `"in-memory (fallback)"`; the
   same value goes to the `oolio_storage_info` metric.
3. `postgres.Migrate` applies embedded `migrations/*.sql` in name order, each in
   its own transaction, recorded in `schema_migrations`.
4. `coupon.LoadIndex(COUPON_INDEX_PATH)`: missing file → `NewUnavailableValidator`
   (rejects all coupons, fail closed); corrupt or wrong format → server exits.
   The loaded code count goes to `oolio_coupon_index_codes` (0 for the fallback).
5. `http.Server` timeouts and the `Shutdown` deadline come from `server.*` in
   the config file.

## Configuration

`config/dev.json`, `stage.json`, `prod.json`, loaded by `internal/config` with
viper (v1.21.0, the newest that keeps `go 1.25.0`). `APP_ENV` picks the file
(default `dev`), `CONFIG_DIR` the folder (default `config`, relative to the
working directory; the Docker image copies it to `/app/config`).

- **Precedence:** file, then the env vars below (explicit `BindEnv`, no
  `AutomaticEnv`). viper ignores an empty env var, so empty = keep the file value.
- **`UnmarshalExact`:** an unknown JSON key is a startup error. Add a struct
  field (with a `mapstructure` tag) before adding a key to the files.
- **`Validate`:** positive durations, port and index path set; outside dev,
  `DATABASE_URL`, `API_KEY`, `JWT_SECRET`, `AUTH_USERNAME`, `AUTH_PASSWORD` must
  be set (files leave them empty) and `fallbackToMemory` must be false. All
  problems are reported at once (`errors.Join`).
- The stage and prod CORS origins are `example.com` placeholders.

| Key | Env override | dev / stage / prod |
| --- | --- | --- |
| `server.port` | `PORT` | 8080 everywhere |
| `server.readHeaderTimeout` … `idleTimeout` | none | 5s, 10s, 10s, 60s (prod idle 120s) |
| `server.shutdownTimeout` | none | 10s / 15s / 30s |
| `log.level` | `LOG_LEVEL` | debug / info / info |
| `cors.allowedOrigin` | `CORS_ALLOWED_ORIGIN` | `*` / placeholder / placeholder |
| `database.url` | `DATABASE_URL` | local DSN / env / env |
| `database.fallbackToMemory` | none | true / false / false |
| `coupon.indexPath` | `COUPON_INDEX_PATH` | `coupons/coupons.idx` |
| `auth.apiKey` | `API_KEY` | `apitest` / env / env |
| `auth.jwtSecret` | `JWT_SECRET` | empty (random per boot) / env / env |
| `auth.jwtTTL` | `JWT_TTL` | 15m / 15m / 10m |
| `auth.username`, `auth.password` | `AUTH_USERNAME`, `AUTH_PASSWORD` | demo, demo1234 / env / env |

Compose sets only `APP_ENV` (default dev) and `DATABASE_URL`; the rest are
`${VAR:-}` pass-throughs so the file wins. Hardcoded Compose defaults once
overrode `prod.json` (TTL 15m instead of 10m, dev credentials in prod), so keep
them as pass-throughs. `deploy/.env` is loaded by Compose automatically.

`discount` (no env override): `defaultPercent` (5 in every file) and a `codes`
list of `{code, percent}` (empty). Percent only, by the owner's choice (no
fixed amounts, caps or minimum spend). A list, not a map: viper lowercases map
keys and coupon codes are case-sensitive. `config.Validate` calls
`discount.Config.Validate`; `cmd/server` builds the `discount.Policy` and passes
it to `order.NewService`.

The `GRAFANA_CLOUD_*` variables in `deploy/.env` are read by Alloy in Compose,
not by the Go server.

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

`docs/diagrams/order.drawio` (HLD, LLD, dry run), `coupon.drawio` (HLD,
package map, LLD, dry run) and `auth.drawio` (HLD, LLD, dry run), with PNG
exports embedded in the README.
