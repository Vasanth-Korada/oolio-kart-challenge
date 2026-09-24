# Workflows

## Run

```bash
make docker-up                 # Postgres 16 + API on :8080 (make docker-down drops the volume)
go run ./cmd/server            # no Postgres → in-memory fallback, warning logged
curl -s localhost:8080/readyz  # {"status":"ready","storage":"postgres"}
```

`POST /order` needs `api_key: apitest`.

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
3. `make postman-test` (newman; currently 15 requests, 22 assertions, all must pass).
4. `docker compose -f deploy/docker-compose.yml down` when done (without `-v`, so
   the volume survives).

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

- Sources: `docs/diagrams/order.drawio`, `coupon.drawio` (one page per view).
  Edit in app.diagrams.net or the VS Code draw.io extension.
- Export every page to cropped PNGs for the README:

  ```bash
  node .claude/skills/oolio-kart/scripts/export-diagrams.mjs docs/diagrams/order.drawio docs/diagrams/order
  ```

  Writes `<prefix>-<page-slug>.png` per page (2x, cropped). Needs Playwright
  (`npm i playwright` in a scratch dir, `NODE_PATH` pointing at it) and network
  access to viewer.diagrams.net. Rename or map outputs to the file names the
  README uses (`order-hld.png`, `order-lld.png`, `order-dry-run.png`,
  `coupon-hld.png`, `coupon-package-map.png`, `coupon-lld.png`,
  `coupon-dry-run.png`).
- Dry-run pages show captured values. If behaviour changes, re-capture on the
  real stack rather than editing numbers by hand.

## Release

Only after the owner approves the version, tag and release text.

1. Move `[Unreleased]` entries under `## [X.Y.Z] - YYYY-MM-DD`; add the link at
   the bottom (`releases/tag/vX.Y.Z`).
2. Bump the README `**Version:**` line.
3. Commit (approved message), push `submission-v2`, wait for green CI.
4. `git tag -a vX.Y.Z <sha> -m "vX.Y.Z: <summary>"` and push it by name:
   `git push origin refs/tags/vX.Y.Z`.
5. `gh release create vX.Y.Z --verify-tag --title "vX.Y.Z: <summary>" --notes-file <that CHANGELOG section> --latest`.
