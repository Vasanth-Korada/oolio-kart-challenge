# Order flow: POST /order

## Path

1. Middleware: `RequestID` (reuses `X-Request-Id` or generates a UUID) →
   `Recover` → `Logging` → `CORS` (OPTIONS preflight → 204 and stops).
2. `ServeMux` matches `POST /order` → `Authenticate` → `RequireScope("create_order")`
   (see [auth.md](auth.md)). Bearer JWT checked first; else `api_key`
   (constant-time; wrong → 403); neither → 401. Token without the scope → 403.
3. `OrderHandler.Create`: JSON decode fails → 400 `invalid_body`. Maps request
   items to `order.Item` and calls `Service.PlaceOrder`.
4. `order.service.PlaceOrder` (`internal/order/service.go`):
   - no items → `ErrEmptyItems`
   - each raw item: quantity ≤ 0 → `ErrInvalidQuantity`; > `MaxItemQuantity`
     (1000) → `ErrQuantityTooLarge`. Checked **before** merging so summing
     duplicates can't overflow `int` into a negative number.
   - `mergeItems`: same `productId` twice → one line, quantities summed, first-seen
     order kept (the `order_items` PK is `(order_id, product_id)`).
   - merged quantity > 1000 → `ErrQuantityTooLarge` (e.g. 600 + 600).
   - for each merged item `products.Get` (one query per item: N+1) →
     `product.ErrNotFound` becomes `ErrProductNotFound`; any other error → 500.
   - `subtotal += price × qty` (prices from the catalog, never the client).
   - `couponCode != ""` and invalid → `ErrInvalidCoupon`.
   - `discount = roundMoney(subtotal × 0.05)` if a coupon was given;
     `total = roundMoney(subtotal − discount)`; id = `idgen.NewUUID()` (v4).
5. `DBRepository.Create`: `pgx.BeginFunc` → insert `orders` (coupon NULL when
   empty) → one insert per item with `unit_price` → commit, or rollback on any
   error.
6. Handler maps errors with `errors.Is`: the five `Err*` sentinels → 422
   `validation_error`; anything else → logged + 500 `internal`. Success → 200.

## Status codes

| Code | When |
| --- | --- |
| 200 | order placed |
| 204 | CORS preflight |
| 400 | malformed JSON |
| 401 | no credentials, or a bad/expired/tampered Bearer token |
| 403 | wrong `api_key`, or a token without `create_order` |
| 405 | wrong method on `/order` (plain text) |
| 422 | empty items, quantity out of 1..1000, unknown product, invalid coupon |
| 500 | unexpected (DB down, panic) |

## Worked example (captured from the real stack)

Request `HAPPYHRS` with items `[{1,2},{3,1},{1,1}]` → merged `[{1,3},{3,1}]` →
subtotal 6.50×3 + 8.00×1 = 27.50 → discount `math.Round(137.5)/100` = 1.38
(half rounds away from zero) → total 26.12. The DB stores `orders` 27.50 / 1.38
/ 26.12 and two `order_items` rows with unit prices 6.50 and 8.00.

## Adding a new validation rule

Add a sentinel `Err*` in `order.go` (it is part of the error contract), return it
wrapped with context (`fmt.Errorf("%w: product %s", ...)`), add it to the 422
list in `httpapi/order_handler.go`, then add service and handler suite cases, a
Postman request, and a README line.

## Metrics

`PlaceOrder` wraps `placeOrder` and reports to `order.Recorder`: `OrderPlaced`
on success, `OrderRejected(reason)` for each validation sentinel (via
`rejectionReason`), and `CouponChecked` whenever a code is looked up. A new
sentinel needs a reason label there too.

## Logging

The service logs with its own `s.logger`, which is not request-scoped, so
`order: created` has no `request_id`; only the access log line does.
