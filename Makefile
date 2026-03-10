.PHONY: build test lint docker-build docker-up docker-down clean

BINARY := crowdsec-capi
MODULE  := github.com/aarnaud/crowdsec-central-api

build:
	go build -trimpath -ldflags="-s -w" -o $(BINARY) .

test:
	go test ./... -count=1

lint:
	go vet ./...
	@which golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed, skipping"

docker-build:
	docker build -t crowdsec-capi:latest .

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

clean:
	rm -f $(BINARY)
