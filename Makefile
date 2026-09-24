.PHONY: build test integration-test vet lint fmt run fetch-coupons build-coupon-index docker-up docker-down postman-test observability-local observability-cloud traffic

build:
	go build ./...

test:
	go test ./... -race -count=1

integration-test:
	go test -tags integration ./... -run TestDBRepository -v

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
	docker compose -f deploy/docker-compose.yml --profile local --profile cloud down -v

observability-local:  # API + local Prometheus (:9090) + Grafana (:3000) with the dashboard
	docker compose -f deploy/docker-compose.yml --profile local up -d --build

observability-cloud:  # API + Alloy pushing metrics to Grafana Cloud (needs deploy/.env)
	docker compose -f deploy/docker-compose.yml --profile cloud up -d --build

traffic:  # 2 minutes of mixed traffic for the dashboard
	./scripts/loadgen.sh 120

postman-test:
	npx --yes newman run postman/oolio-kart-challenge.postman_collection.json -e postman/local.postman_environment.json
