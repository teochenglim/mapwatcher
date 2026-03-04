BINARY    := bin/mapwatch
MODULE    := github.com/teochenglim/mapwatch
GO        := go
GOFLAGS   := -trimpath
LDFLAGS   := -s -w
VERSION   := $(strip $(shell cat VERSION))

.PHONY: all build run test test-verbose lint tidy docker-build docker-push \
        tag release demo clean help

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

## demo: run full stack using Dockerfile-local (no GeoJSON downloads)
demo:
	docker compose -f docker-compose.yml -f docker-compose-local.yml up --build
	@echo "→ MapWatch demo running at http://localhost:8080"

## docker-build: build Docker image tagged with VERSION and latest
docker-build:
	docker build -t ghcr.io/teochenglim/mapwatch:$(VERSION) -t ghcr.io/teochenglim/mapwatch:latest .

## docker-push: push Docker image to ghcr.io (requires docker login)
docker-push:
	docker push ghcr.io/teochenglim/mapwatch:$(VERSION)
	docker push ghcr.io/teochenglim/mapwatch:latest

## tag: create an annotated git tag from the VERSION file
tag:
	@echo "Tagging $(VERSION) (from VERSION file)…"
	git tag -a $(VERSION) -m "Release $(VERSION)"
	@echo "Push with: make release"

## release: tag + push to GitHub — triggers the Release GitHub Action
release: tag
	@echo "Pushing $(VERSION) to GitHub…"
	git push origin $(VERSION)
	@echo "GitHub Actions will build and publish ghcr.io/teochenglim/mapwatch:$(VERSION)"

## clean: remove build artifacts
clean:
	rm -rf bin/

## help: list all targets
help:
	@grep -E '^## ' Makefile | sed 's/## //'
