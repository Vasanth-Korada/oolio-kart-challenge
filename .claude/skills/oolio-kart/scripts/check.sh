#!/usr/bin/env bash
# Runs the same checks as CI (.github/workflows/ci.yml), from the repo root.
# Exits non-zero on the first failure.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

step() { printf '\n==> %s\n' "$*"; }

step "gofmt"
unformatted="$(gofmt -l .)"
if [ -n "$unformatted" ]; then
  echo "not gofmt-formatted:"; echo "$unformatted"; exit 1
fi

step "go vet"
go vet ./...

step "golangci-lint config verify"
golangci-lint config verify

step "golangci-lint run"
golangci-lint run --build-tags=integration ./...

step "go build"
go build ./...

step "go test -race"
go test ./... -race -count=1

printf '\nall checks passed (branch: %s)\n' "$(git branch --show-current)"
