---
name: oolio-kart
description: Project context and working rules for the oolio-kart-challenge Go backend (food-ordering API with a 313M-code coupon index). Use this skill for any work in this repo, even when it isn't named: adding a feature or endpoint, fixing a bug, touching the coupon index or the order flow, writing or converting tests, updating README, CHANGELOG or draw.io diagrams, committing, pushing, tagging or releasing, rebuilding coupons.idx, or answering "why was it built this way" questions about the design. It holds the branch policy, commit approval rule, conventions, known limitations and the gotchas already hit, so read it before changing anything.
---

# oolio-kart-challenge

Go backend for Oolio's food-ordering OpenAPI spec: products, orders, and coupon
validation against ~313M candidate codes via an offline-built index. It was an
interview take-home; `main` is what was submitted, and all later work lives on
`submission-v2`. The repo's `CLAUDE.md` holds the general guidelines; this skill
adds the project knowledge and the rules learned while working on it.

## Rules that matter most

These come from explicit decisions by the repo owner. Breaking them has real
cost (the submission is under review), so treat them as fixed.

1. **Never commit, push or merge to `main`.** It is the submitted code
   (`8028b8e`) plus one owner commit adding `CLAUDE.md`. Work on
   `submission-v2`. Never merge PR #1 (`submission-v2` → `main`).
2. **Show the exact commit message and wait for approval before committing.**
   The owner reviews every message and sometimes asks to squash work into one
   commit. Tags and GitHub releases get the same treatment.
3. **Conventional Commits**, one concern per commit (`feat:`, `fix:`,
   `refactor:`, `test:`, `docs:`, `ci:`, `perf:`). No `Co-Authored-By: Claude`
   trailers.
4. **Force-push only `submission-v2`, only with explicit approval**, and always
   with `--force-with-lease`.
5. **Verify before calling something done:** the full check script, and for
   behaviour changes a live run on Docker + Postgres (see workflows).

## Where to look

| Task | Read |
| --- | --- |
| Understand packages, layers, request path, config, DB schema | [references/architecture.md](references/architecture.md) |
| Anything touching coupons, `coupons.idx`, `cmd/buildindex` | [references/coupon-index.md](references/coupon-index.md) |
| `POST /order`: validation, pricing, errors, status codes | [references/order-flow.md](references/order-flow.md) |
| Git, commits, tests, README/CHANGELOG style, versioning, CI | [references/conventions.md](references/conventions.md) |
| Run, test, live-verify, rebuild the index, diagrams, release | [references/workflows.md](references/workflows.md) |
| Metrics, Grafana dashboard, Alloy / Grafana Cloud | [references/observability.md](references/observability.md) |
| What is known to be weak, and the planned fix for each | [references/known-limitations.md](references/known-limitations.md) |

## Quick map

```
cmd/server         wiring, graceful shutdown, Postgres-or-memory fallback
cmd/buildindex     offline: coupons/raw/*.gz → coupons/coupons.idx
internal/httpapi   router, middleware, handlers, error envelope
internal/order     PlaceOrder: validate, merge, price, coupon, store (1 tx)
internal/product   catalog (Postgres or in-memory seed)
internal/coupon    Validator, Index, Source, build pipeline, CPX2 file format
internal/platform  pgx pool + migrations, JSON logger, Prometheus metrics, UUID v4
migrations/        embedded SQL; docs/diagrams/ draw.io sources + PNGs
deploy/            Compose (profiles: local, cloud), Alloy, Prometheus, Grafana dashboard
```

## Before you push

```bash
.claude/skills/oolio-kart/scripts/check.sh
```

It runs exactly what CI runs (gofmt, vet, golangci-lint config verify + run,
build, race tests). CI also runs on every push to `submission-v2`; check it with
`gh run list --branch submission-v2`.

## Every change also updates

- **Tests** in `testify/suite` style (see conventions), including a case that
  fails without the change.
- **README.md** when behaviour, setup, API or usage changes.
- **CHANGELOG.md** under `[Unreleased]`.
- **docs/diagrams** when a drawn flow changes (regenerate PNGs with the export
  script).
- **Postman collection** when an API behaviour or error path changes.
