# Observability

```
Go API /metrics → Grafana Alloy (every 15s) → Grafana Cloud Prometheus → dashboard
```

## Code

- `internal/platform/metrics`: the only package importing Prometheus. `metrics.New()`
  builds collectors on a private registry (tests create independent instances),
  plus Go runtime and process collectors. `Handler()` serves `/metrics`.
- Consumers own small interfaces that `*metrics.Metrics` satisfies (checked at
  compile time in `metrics_test.go`):
  - `httpapi.HTTPMetrics`: `RequestStarted`, `RequestFinished(method, route, status, took)`
  - `order.Recorder`: `OrderPlaced`, `OrderRejected(reason)`, `CouponChecked(valid)`
- `httpapi.Metrics` middleware resolves the route **before** serving with
  `mux.Handler(r)`, because inner middleware passes request copies and the mux's
  pattern never reaches the outer layer. Unmatched → `unmatched`; `/metrics` itself
  is not recorded. Never label by raw path (unbounded series).
- `cmd/server` sets `storage_info` and `coupon_index_codes` at startup.

## Metrics (prefix `oolio_`)

`http_requests_total{method,route,status}`, `http_request_duration_seconds{method,route}`,
`http_requests_in_flight`, `orders_placed_total{coupon}`, `orders_rejected_total{reason}`,
`orders_total_amount` (histogram, dollars), `coupon_checks_total{result}`,
`coupon_index_codes`, `storage_info{mode}`, plus `go_*` and `process_*`.

Adding a metric: add the collector in `metrics.New`, a method on `Metrics`, the
method on the consumer's interface (and its no-op), a panel in the dashboard
JSON, a row in the README metrics table, and a suite test that scrapes it.

## Deploy

- `deploy/docker-compose.yml` profiles: `local` (Prometheus `:9090`, Grafana `:3000`,
  anonymous admin, dashboard provisioned as home) and `cloud` (Alloy, UI on `:12345`).
- `deploy/alloy/config.alloy` reads `GRAFANA_CLOUD_PROM_URL`, `GRAFANA_CLOUD_PROM_USER`,
  `GRAFANA_CLOUD_API_TOKEN` from `deploy/.env` (gitignored). Never write the token
  into any file or command yourself; the owner fills `deploy/.env`. Check it with
  `grep -q '^VAR=.'`, never by printing values.
- Grafana Cloud stack: region ap-south-1, Prometheus `prometheus-prod-43`
  (remote write `https://prometheus-prod-43-prod-ap-south-1.grafana.net/api/prom/push`).
- Dashboard: `deploy/grafana/dashboards/oolio-kart.json`, uid `oolio-kart-api`, with a
  `datasource` variable, so it imports into any Grafana. Import via Dashboards →
  New → Import.

## Verify

1. `make observability-local` (and/or `make observability-cloud`), wait for `/readyz`.
2. `make traffic` (or `scripts/loadgen.sh <seconds>`).
3. `curl -s localhost:8080/metrics | grep ^oolio_`; Prometheus targets at
   `localhost:9090/api/v1/targets` must be `up`.
4. Alloy: `docker compose -f deploy/docker-compose.yml logs alloy` shows no remote
   write errors; in Grafana Cloud Explore, `oolio_http_requests_total` has data.
5. Stop everything with `make docker-down` (all profiles).
