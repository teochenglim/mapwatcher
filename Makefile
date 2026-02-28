BINARY    := bin/mapwatch
MODULE    := github.com/teochenglim/mapwatch
GO        := go
GOFLAGS   := -trimpath
LDFLAGS   := -s -w
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: all build run test lint docker-build docker-push clean tidy

all: build

## build: compile the binary
build:
	@mkdir -p bin
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) -X main.version=$(VERSION)" -o $(BINARY) ./cmd/mapwatch

## run: build and run the server
run: build
	./$(BINARY) serve

## test: run all tests (unit + integration in tests/)
test:
	$(GO) test -race -count=1 ./tests/...

## test-verbose: run tests with verbose output
test-verbose:
	$(GO) test -race -count=1 -v ./tests/...

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## tidy: tidy and verify go modules
tidy:
	$(GO) mod tidy
	$(GO) mod verify

## docker-build: build Docker image
docker-build:
	docker build -t ghcr.io/teochenglim/mapwatch:$(VERSION) -t ghcr.io/teochenglim/mapwatch:latest .

## docker-push: push Docker image (requires docker login to ghcr.io)
docker-push:
	docker push ghcr.io/teochenglim/mapwatch:$(VERSION)
	docker push ghcr.io/teochenglim/mapwatch:latest

## clean: remove build artifacts
clean:
	rm -rf bin/
