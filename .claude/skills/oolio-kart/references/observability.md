# Observability

```
Go API /metrics → Grafana Alloy (every 15s) → Grafana Cloud Prometheus → dashboard
```

Added in v1.2.0.

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
- Live dashboard in Grafana Cloud: https://brownvalley96.grafana.net/goto/sd9ppg (linked in the README).
- Dashboard: `deploy/grafana/dashboards/oolio-kart.json`, uid `oolio-kart-api`, with a
  `datasource` variable, so it imports into any Grafana. Import via Dashboards →
  New → Import.

## Editing the dashboard

- The repo JSON is the source of truth. Edits made in the Grafana Cloud UI don't
  reach the repo: export (Share → Export → Save to file, with "Export for sharing
  externally" off so the `datasource` variable stays) and replace
  `deploy/grafana/dashboards/oolio-kart.json`, then re-import to update Cloud.
- Local Grafana reloads the provisioned file from disk within about 10 seconds.
- After any change, run every panel query against real data:

  ```bash
  python3 .claude/skills/oolio-kart/scripts/check-dashboard.py
  ```

  It fills in `$job`, `$__rate_interval` and `$__range` and queries the local
  Prometheus (`localhost:9090` by default; pass another URL as the second
  argument). Every query must return data after `make traffic`.

## README screenshot

`docs/images/grafana-dashboard.png`. Grafana only renders panels inside the
viewport, so capture with a tall viewport (about 1600×3900) at
`localhost:3000/d/oolio-kart-api/oolio-kart-api?kiosk&theme=light&from=now-10m`
while `make traffic` is running (rates read 0 once traffic stops), then crop the
empty space below the last row.

## Verify

1. `make observability-local` (and/or `make observability-cloud`), wait for `/readyz`.
2. `make traffic` (or `scripts/loadgen.sh <seconds>`).
3. `curl -s localhost:8080/metrics | grep ^oolio_`; Prometheus targets at
   `localhost:9090/api/v1/targets` must be `up`.
4. Alloy: `curl -s localhost:12345/metrics | grep prometheus_remote_storage_samples`
   shows `samples_total` growing and `samples_failed_total` at 0. A single
   "Skipping resharding" warning at startup is harmless. In Grafana Cloud Explore
   (data source `grafanacloud-brownvalley96-prom`), `oolio_http_requests_total`
   has data. Filter any Alloy log output through `sed` to mask `glc_` tokens.
5. Stop everything with `make docker-down` (all profiles).
