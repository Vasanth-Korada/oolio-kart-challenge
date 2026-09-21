#!/usr/bin/env bash
# Downloads Oolio's three raw coupon files into coupons/raw/.
#
# These files are large (~650-740MB each) and, in practice, plain
# downloads over an unreliable connection can silently stall or
# truncate mid-transfer without curl reporting a non-zero exit code —
# hit first-hand while building this project. Trusting the reported
# "success" of a truncated download would mean buildindex silently
# processes incomplete data and under-counts valid coupons.
#
# So this script verifies every download's final size against the
# origin's authoritative Content-Length header and resumes (rather than
# restarting) until it matches, instead of trusting curl's exit code
# alone.
set -euo pipefail

BASE_URL="https://orderfoodonline-files.s3.ap-southeast-2.amazonaws.com"
OUT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/coupons/raw"
FILES=(couponbase1 couponbase2 couponbase3)
MAX_ATTEMPTS=30

mkdir -p "$OUT_DIR"

for name in "${FILES[@]}"; do
  url="${BASE_URL}/${name}.gz"
  dest="${OUT_DIR}/${name}.gz"
  tmp="${dest}.part"

  expected=$(curl -sI "$url" | grep -i '^content-length:' | tr -d '\r' | awk '{print $2}')
  if [ -z "$expected" ]; then
    echo "fetch_coupons: could not read Content-Length for $name, aborting" >&2
    exit 1
  fi

  if [ -f "$dest" ] && [ "$(stat -f%z "$dest" 2>/dev/null || stat -c%s "$dest")" = "$expected" ]; then
    echo "$name: already complete ($expected bytes), skipping"
    continue
  fi

  attempt=0
  actual=$(stat -f%z "$tmp" 2>/dev/null || stat -c%s "$tmp" 2>/dev/null || echo 0)
  while [ "$actual" -lt "$expected" ] && [ "$attempt" -lt "$MAX_ATTEMPTS" ]; do
    attempt=$((attempt + 1))
    echo "$name: attempt $attempt, resuming from $actual / $expected bytes"
    curl -sL -C - --retry 5 --retry-delay 3 -o "$tmp" "$url" || true
    actual=$(stat -f%z "$tmp" 2>/dev/null || stat -c%s "$tmp" 2>/dev/null || echo 0)
  done

  if [ "$actual" -ne "$expected" ]; then
    echo "$name: FAILED after $attempt attempts ($actual / $expected bytes) — rerun this script to resume" >&2
    exit 1
  fi

  mv "$tmp" "$dest"
  echo "$name: OK ($actual bytes)"
done

echo "all coupon files downloaded to $OUT_DIR"
