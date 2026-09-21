.PHONY: build test integration-test vet fmt run fetch-coupons build-coupon-index docker-up docker-down

build:
	go build ./...

test:
	go test ./... -race -count=1

integration-test:
	go test -tags integration ./... -run TestPostgresRepository -v

vet:
	go vet ./...

fmt:
	gofmt -l .

run:
	go run ./cmd/server

fetch-coupons:
	./scripts/fetch_coupons.sh

build-coupon-index:
	go run ./cmd/buildindex

docker-up:
	docker compose -f deploy/docker-compose.yml up --build

docker-down:
	docker compose -f deploy/docker-compose.yml down -v
