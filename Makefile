.PHONY: build test integration-test vet lint fmt run fetch-coupons build-coupon-index docker-up docker-down postman-test

build:
	go build ./...

test:
	go test ./... -race -count=1

integration-test:
	go test -tags integration ./... -run TestPostgresRepository -v

vet:
	go vet ./...

lint:
	golangci-lint run --build-tags=integration ./...

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

postman-test:
	npx --yes newman run postman/oolio-kart-challenge.postman_collection.json -e postman/local.postman_environment.json
