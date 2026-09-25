# Known limitations

Found in self-review. This is the full list; the README's Known Limitations
table shows the main ones (it leaves out the one worker per file, the `"01"` id and the empty CORS origin rows). When one is fixed, remove it here and from the README if it is listed there, and add a
CHANGELOG entry.

| Limitation | Where | Planned fix |
| --- | --- | --- |
| Money is `float64` in Go (exact `NUMERIC` in Postgres) | `order/service.go`, `roundMoney` | `int64` cents end to end |
| No `Idempotency-Key`: a client retry creates a duplicate order | `POST /order` | Header + unique constraint, return the stored order |
| Committed index holds valid codes in plain text | `coupons/coupons.idx` | Build in CI from a private source |
| Service logs lack `request_id` | `order.service` uses `s.logger` | Request-scoped logger from context |
| A panic skips the access log line | middleware order in `router.go` | Put `Logging` outside `Recover` |
| No request body size limit | `OrderHandler.Create` | `http.MaxBytesReader` |
| Migrations have no lock across replicas | `postgres.Migrate` | `pg_advisory_lock` |
| `/metrics` is public on the API port | `httpapi.NewRouter` | Separate internal port or network policy |
| One worker per coupon file | `coupon.Build` | Split each file by gzip member / byte range |
| `"01"` works on `GET /product/01` but not as an order `productId` | handler vs service | Normalise ids in one place |
| `CORS_ALLOWED_ORIGIN=""` can't disable CORS (viper ignores empty env vars) | `internal/config` | Set `cors.allowedOrigin` to `""` in the file, or `AllowEmptyEnv` |
| One demo user from env vars, no `users` table | `auth.MemoryUserStore` | Postgres `users` table behind `auth.UserStore` |
| No refresh tokens or revocation | `auth.JWTManager` | Refresh token rotation; `jti` denylist |
| No rate limit on `POST /auth/token` | `AuthHandler.Token` | Per-IP and per-user limiter |

Fixed so far (don't re-add): N+1 order queries (batched, 5 statements per order),
huge quantity returned 500 (capped at 1000 → 422),
coupon checked after the product lookup (now before it, 0 SQL for a bad coupon),
flat 5% discount hardcoded (now `discount` percents in config, default 5%),
truncated gzip line indexed as a code, invalid `.golangci.yml` v1 key.
