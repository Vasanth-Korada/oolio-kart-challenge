# Known limitations

Found in self-review. This is the full list; the README's Known Limitations
table shows the main ones (everything except the last four rows). When one is
fixed, remove it here and from the README if it is listed there, and add a
CHANGELOG entry.

| Limitation | Where | Planned fix |
| --- | --- | --- |
| Money is `float64` in Go (exact `NUMERIC` in Postgres) | `order/service.go`, `roundMoney` | `int64` cents end to end |
| One `SELECT` per item, one `INSERT` per item (N+1) | `service.PlaceOrder`, `order.DBRepository.Create` | `WHERE id = ANY($1)`; `pgx.Batch` or `CopyFrom` |
| No `Idempotency-Key`: a client retry creates a duplicate order | `POST /order` | Header + unique constraint, return the stored order |
| Flat 5% discount hardcoded | `order.CouponDiscountRate` | `DiscountPolicy` interface per coupon |
| Committed index holds valid codes in plain text | `coupons/coupons.idx` | Build in CI from a private source |
| Service logs lack `request_id` | `order.service` uses `s.logger` | Request-scoped logger from context |
| A panic skips the access log line | middleware order in `router.go` | Put `Logging` outside `Recover` |
| No request body size limit | `OrderHandler.Create` | `http.MaxBytesReader` |
| Migrations have no lock across replicas | `postgres.Migrate` | `pg_advisory_lock` |
| Coupon checked after the product lookups | `service.PlaceOrder` | Check it first (in-memory, cheap) |
| One worker per coupon file | `coupon.Build` | Split each file by gzip member / byte range |
| `"01"` works on `GET /product/01` but not as an order `productId` | handler vs service | Normalise ids in one place |
| `CORS_ALLOWED_ORIGIN=""` can't disable CORS | `config.getEnv` | Distinguish unset from empty |

Fixed so far (don't re-add): huge quantity returned 500 (capped at 1000 → 422),
truncated gzip line indexed as a code, invalid `.golangci.yml` v1 key.
