# Conventions

## Git

- `main` is frozen (submitted code + the owner's `CLAUDE.md` commit). Never
  commit, push, merge or open PRs into it. PR #1 (`submission-v2` → `main`) was
  opened by the owner; leave it alone.
- All work goes to `submission-v2`. Pull (`--ff-only`) before starting.
- Before each commit, show the owner the exact message (subject + body) and wait
  for a yes. They may reword it or ask to squash several changes into one.
- Conventional Commits: `type: imperative summary`, then a short body saying why.
  One concern per commit; if two changes touch the same file, stage them
  separately (save a patch and `git apply --cached` works well).
- No `Co-Authored-By: Claude` trailer, even if a harness suggests one; the owner
  asked for this explicitly.
- Force-push only `submission-v2`, only when approved, only with
  `--force-with-lease=submission-v2:<expected-sha>`.
- Check `git status` before starting: the owner sometimes commits or edits files
  themselves (e.g. `CLAUDE.md`). Never overwrite their uncommitted work.
- The owner also runs `make docker-up` etc. in their own terminal. Before starting
  Compose, check `ps` for another `docker compose` process on this project; two
  of them block each other. Never kill theirs; ask them to stop it.
- Scan new commits for secrets before pushing, e.g.
  `git diff <base>..HEAD | grep -cE 'glc_[A-Za-z0-9]{10,}'` must print 0.

## Go code

- Dependencies must keep `go.mod` at `go 1.25.0` (CI uses Go 1.25). `@latest`
  of `golang.org/x/*` may require a newer Go and bump the directive: pin an older
  version (e.g. `x/crypto v0.55.0`) and check `git diff go.mod` after `go get`.

- Idiomatic Go, `gofmt`, small packages, sentinel errors compared with `errors.Is`.
- Descriptive names (owner's request, 2026-09-25): no single-letter locals, loop
  variables or parameters in non-test code (`fileIndex`, `cursors`, `lastUnique`,
  not `i`, `idxs`, `j`). Kept: method receivers, `w`/`r` in handlers, `ctx`, `err`,
  and `t`/`s`/`tt` in tests. Don't name a variable after an imported package in
  the same file (`httpapi` uses `item`, not `product`). `gopls rename` does it
  scope-safely.
- Interface first, but only at real seams (a consumer needs it, or tests need a
  fake). Define interfaces where they are consumed.
- GoDoc on every package and exported identifier; the comment starts with the
  identifier's name and explains behaviour or contract, not the obvious.
- Parameterized SQL only. `//nolint` only with a reason on the same line.

## Tests

- `testify/suite` for any test file you create or touch (converted so far:
  `internal/coupon`, `internal/platform/metrics`, `order/service_test.go`,
  `httpapi/order_handler_test.go`, `httpapi/metrics_test.go`,
  `httpapi/middleware_test.go`, `httpapi/contract_test.go`,
  `httpapi/auth_handler_test.go`, `internal/auth`, `internal/config`,
  `product/service_test.go`). One suite per unit: add tests to the unit's
  existing suite file rather than a new `*_test.go` beside it.
  Other packages still use plain `t.Run` tables; convert them when touched.
- Table-driven cases inside suite methods (`s.Run(tt.name, ...)`).
- For a bug fix, show the new test fails without the fix (temporarily revert, run,
  restore) before relying on it.
- Postgres tests are behind `//go:build integration` and need `DATABASE_URL`.
- `index_real_data_test.go` checks the committed index; it skips if missing.

## README

- Short, one-line bullets and tables; no long paragraphs.
- No em dashes. Don't mention Bloom filters (owner's choice; the design uses an
  exact index).
- Update it whenever behaviour, setup, API or usage changes; keep the Known
  Limitations table current (remove a row when fixed).
- Claims must match the code; check before writing (the CORS default and the
  `discounts` field were both documented wrongly once).
- Diagrams are PNG exports of `docs/diagrams/*.drawio`; the detailed LLD and dry
  run pages sit in `<details>` blocks.
- Diagram style (owner's request): easy to understand at a glance. A box is a
  bold name plus at most one short line; decisions are short questions; errors
  are red boxes with the status and a few words; no code blocks; at most one
  small note per page; aim for about 150 words per page or fewer.

## CHANGELOG and versions

- Keep a Changelog format, Semantic Versioning, newest first, `[Unreleased]` at
  the top for work not yet released.
- The HTTP API and CLI are the public interface. `internal/` packages and the
  `coupons.idx` format are not, so changing them is minor or patch, not major
  (the owner chose 1.1.0 over 2.0.0 for the index format change).
- Released: `v1.0.0` (`8028b8e`, submission), `v1.1.0` (`c465f40`, raw keys +
  refactor) `v1.1.1` (quantity cap + GoDoc), `v1.2.0` (Prometheus metrics +
  Grafana), `v1.3.0` (JWT auth) and `v1.4.0` (config files, batched order
  queries, simpler diagrams), all annotated tags with GitHub releases.

## CI (`.github/workflows/ci.yml`)

- Runs on push to `main` and `submission-v2`, and on PRs.
- Steps: gofmt check, `go vet`, golangci-lint via `golangci-lint-action@v9` pinned
  to v2.13.2 (matches the local version), build, `go test ./... -race -count=1`.
- The action runs `golangci-lint config verify` first; `.golangci.yml` must use v2
  keys (`linters.default: none`, not `disable-all`).
- `actions/setup-go` v6+ sets `GOTOOLCHAIN=local`, so tools needing a newer Go
  can't self-download; that is why the linter is pinned via the action.
