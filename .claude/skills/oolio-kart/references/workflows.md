# Workflows

## Run

```bash
make docker-up                 # Postgres 16 + API on :8080
make observability-local       # + Prometheus :9090 + Grafana :3000 (dashboard preloaded)
make observability-cloud       # + Alloy pushing to Grafana Cloud (needs deploy/.env)
make traffic                   # 2 min of mixed traffic for the dashboard
make docker-down               # stops every profile and drops the volume
go run ./cmd/server            # APP_ENV=dev: no Postgres → in-memory fallback, warning logged
curl -s localhost:8080/readyz  # {"status":"ready","storage":"postgres"}
```

`POST /order` needs `api_key: apitest`, or `Authorization: Bearer <token>` from
`POST /auth/token` with `{"username":"demo","password":"demo1234"}`.

## Check (same as CI)

```bash
.claude/skills/oolio-kart/scripts/check.sh
```

Add `make integration-test` (needs `DATABASE_URL`, e.g. with `make docker-up`
running) when repositories or SQL change.

## Live verification for behaviour changes

Unit tests aren't enough for anything touching HTTP, SQL or validation:

1. `docker compose -f deploy/docker-compose.yml up -d --build` and wait for `/readyz`.
2. `curl` the happy path and each changed error path; check status and body.
3. `make postman-test` (newman; currently 19 requests, 32 assertions, all must pass).
4. `docker compose -f deploy/docker-compose.yml down` when done (without `-v`, so
   the volume survives).
5. If metrics changed, also `curl -s localhost:8080/metrics | grep ^oolio_` and
   run the dashboard query check (see observability).

## Rebuild the coupon index

```bash
make fetch-coupons          # ~2.1 GB into coupons/raw/, resumable
make build-coupon-index     # ~5 min; overwrites coupons/coupons.idx
```

With unchanged code the result must be byte-identical to the committed file.
After changing build or format code, also rebuild into a scratch path
(`go run ./cmd/buildindex -out /tmp/x.idx`) and `cmp` it. Run it alone: tests or
lint running alongside can make it 3× slower.

## Diagrams

- Diagrams are generated, not hand-drawn. Edit
  `.claude/skills/oolio-kart/scripts/diagrams/generate.py`, then run:

  ```bash
  NODE_PATH=<dir with playwright>/node_modules .claude/skills/oolio-kart/scripts/diagrams/build.sh
  ```

  It rewrites `docs/diagrams/{order,coupon,auth}.drawio` and the ten README PNGs
  (`order-hld`, `order-lld`, `order-dry-run`, `coupon-hld`, `coupon-package-map`,
  `coupon-lld`, `coupon-dry-run`, `auth-hld`, `auth-lld`, `auth-dry-run`). Hand
  edits to the `.drawio` files are overwritten on the next run.
- Needs Playwright matching the browsers in `~/Library/Caches/ms-playwright`
  (`npm i playwright@latest` in a scratch dir) and network access to
  viewer.diagrams.net; the export retries once if the viewer stalls.
- Copies live in `/Users/vasanth/Desktop/oolio/diagrams` (`order-placement.drawio`,
  `coupon-buildindex.drawio`, `auth-jwt.drawio`); copy the new files there too.
- Dry-run pages show captured values. If behaviour changes, re-capture on the
  real stack and update the numbers in `generate.py`.
- Always look at every exported PNG before committing: overlapping arrows and
  labels are the usual problem, fixed with `exitX/exitY/entryX/entryY` in `E(...)`.

## Release

Only after the owner approves the version, tag and release text.

1. Move `[Unreleased]` entries under `## [X.Y.Z] - YYYY-MM-DD`; add the link at
   the bottom (`releases/tag/vX.Y.Z`).
2. Bump the README `**Version:**` line.
3. Commit (approved message), push `submission-v2`, wait for green CI.
4. `git tag -a vX.Y.Z <sha> -m "vX.Y.Z: <summary>"` and push it by name:
   `git push origin refs/tags/vX.Y.Z`.
5. `gh release create vX.Y.Z --verify-tag --title "vX.Y.Z: <summary>" --notes-file <that CHANGELOG section> --latest`.
