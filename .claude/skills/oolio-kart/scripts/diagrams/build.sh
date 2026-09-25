#!/usr/bin/env bash
# Regenerates every diagram and its README PNG exports.
# Needs Python 3, Node, and Playwright (set NODE_PATH to its node_modules).
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
HERE=.claude/skills/oolio-kart/scripts
OUT=docs/diagrams
TMP="$(mktemp -d)"

python3 "$HERE/diagrams/generate.py" "$OUT"
for f in order coupon auth; do
  node "$HERE/export-diagrams.mjs" "$OUT/$f.drawio" "$TMP/$f" >/dev/null
done

# export-diagrams names files after the page titles; map them to README names.
while read -r src dst; do
  cp "$TMP/$src.png" "$OUT/$dst.png"
done <<'MAP'
order-hld-order-placement order-hld
order-lld-order-placement-flow order-lld
order-dry-run-real-request order-dry-run
coupon-hld-coupon-validation coupon-hld
coupon-package-map-interfaces coupon-package-map
coupon-lld-buildindex-internals coupon-lld
coupon-lld-dry-run coupon-dry-run
auth-hld-jwt-auth auth-hld
auth-lld-token-issue-and-verify auth-lld
auth-dry-run-real-requests auth-dry-run
MAP
rm -rf "$TMP"
echo "diagrams regenerated in $OUT"
