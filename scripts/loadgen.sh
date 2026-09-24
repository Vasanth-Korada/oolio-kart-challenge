#!/usr/bin/env bash
# Sends a mix of realistic traffic to the API so the Grafana dashboard has
# something to show: catalog reads, orders with and without coupons, orders
# with a JWT from /auth/token, and every kind of rejection. Usage: scripts/loadgen.sh [seconds] [base-url]
set -euo pipefail

DURATION="${1:-120}"
BASE="${2:-http://localhost:8080}"
KEY="${API_KEY:-apitest}"
END=$((SECONDS + DURATION))

USER_NAME="${AUTH_USERNAME:-demo}"
USER_PASS="${AUTH_PASSWORD:-demo1234}"

order() { # $1 = JSON body, $2 = api key (optional override)
  curl -s -o /dev/null -X POST "$BASE/order" \
    -H 'Content-Type: application/json' -H "api_key: ${2:-$KEY}" -d "$1"
}

login() { # prints an access token, or nothing on failure
  curl -s -X POST "$BASE/auth/token" -H 'Content-Type: application/json' \
    -d "{\"username\":\"$USER_NAME\",\"password\":\"$1\"}" |
    sed -n 's/.*"accessToken":"\([^"]*\)".*/\1/p'
}

order_jwt() { # $1 = JSON body, $2 = token
  curl -s -o /dev/null -X POST "$BASE/order" \
    -H 'Content-Type: application/json' -H "Authorization: Bearer $2" -d "$1"
}

TOKEN="$(login "$USER_PASS")"

echo "sending traffic to $BASE for ${DURATION}s"
n=0
while [ "$SECONDS" -lt "$END" ]; do
  id=$((RANDOM % 12 + 1)) # 11 and 12 don't exist → 404s
  curl -s -o /dev/null "$BASE/product"
  curl -s -o /dev/null "$BASE/product/$id"
  qty=$((RANDOM % 4 + 1))
  case $((RANDOM % 10)) in
    0 | 1 | 2) order "{\"items\":[{\"productId\":\"$((RANDOM % 10 + 1))\",\"quantity\":$qty}],\"couponCode\":\"HAPPYHRS\"}" ;;
    3 | 4 | 5) order "{\"items\":[{\"productId\":\"$((RANDOM % 10 + 1))\",\"quantity\":$qty},{\"productId\":\"3\",\"quantity\":1}]}" ;;
    6) order '{"items":[{"productId":"1","quantity":1}],"couponCode":"SUPER100"}' ;;
    7) order '{"items":[{"productId":"10","quantity":20000000}]}' ;;
    8) order '{"items":[{"productId":"999","quantity":1}]}' ;;
    9) order '{"items":[{"productId":"1","quantity":1}]}' wrong-key ;;
  esac
  case $((RANDOM % 6)) in
    0 | 1 | 2) order_jwt "{\"items\":[{\"productId\":\"$((RANDOM % 10 + 1))\",\"quantity\":$qty}]}" "$TOKEN" ;;
    3) order_jwt '{"items":[{"productId":"2","quantity":1}]}' "${TOKEN}tampered" ;;
    4) login wrong-password >/dev/null ;;
    5) TOKEN="$(login "$USER_PASS")" ;;
  esac
  curl -s -o /dev/null "$BASE/product/abc"
  n=$((n + 1))
  sleep 0.2
done
echo "done: $n rounds"
