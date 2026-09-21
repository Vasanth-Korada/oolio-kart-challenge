.PHONY: build test vet fmt run fetch-coupons build-coupon-index docker-up docker-down

build:
	go build ./...

test:
	go test ./... -race -count=1

vet:
	go vet ./...

fmt:
	gofmt -l .

run:
	go run ./cmd/server

# Downloads the three raw coupon files from Oolio's S3 bucket into
# coupons/raw/ (~2.1GB total, not committed — see .gitignore). Verifies
# each download's size against S3's Content-Length and resumes instead
# of silently indexing a truncated file, since these files are large
# enough that plain downloads can stall mid-transfer.
fetch-coupons:
	./scripts/fetch_coupons.sh

# Builds coupons/coupons.idx from the raw files. Run once after
# fetch-coupons; the resulting index is small and IS committed.
build-coupon-index:
	go run ./cmd/buildindex

docker-up:
	docker compose -f deploy/docker-compose.yml up --build

docker-down:
	docker compose -f deploy/docker-compose.yml down -v
