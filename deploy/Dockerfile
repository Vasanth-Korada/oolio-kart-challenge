# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.25-alpine AS builder
WORKDIR /src

# Cache dependency downloads separately from source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- runtime stage ----
# Distroless: no shell, no package manager, minimal attack surface — the
# binary is statically linked (CGO_ENABLED=0) so it needs nothing else.
# Runs as the built-in nonroot user.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

COPY --from=builder /out/server ./server
# coupons.idx is a small, committed build artifact (~136 bytes for the
# current data) — not rebuilt in the image, see README for why.
COPY coupons/coupons.idx ./coupons/coupons.idx

EXPOSE 8080
ENTRYPOINT ["./server"]
