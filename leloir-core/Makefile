.PHONY: help build build-all test test-short test-race vet lint tidy fmt clean \
	docker-build docker-push \
	controlplane mcp-gateway webhook-receiver

VERSION ?= dev
LDFLAGS = -s -w -X main.version=$(VERSION)
GOFLAGS = -trimpath

help:
	@echo "Leloir Control Plane — make targets:"
	@echo ""
	@echo "  Build:"
	@echo "    make build                 Build all three binaries into ./bin/"
	@echo "    make controlplane          Build only leloir-controlplane"
	@echo "    make mcp-gateway           Build only leloir-mcp-gateway"
	@echo "    make webhook-receiver      Build only leloir-webhook-receiver"
	@echo ""
	@echo "  Test:"
	@echo "    make test                  Run all tests"
	@echo "    make test-short            Skip long-running tests"
	@echo "    make test-race             Run with race detector"
	@echo ""
	@echo "  Quality:"
	@echo "    make vet                   go vet"
	@echo "    make lint                  golangci-lint (requires install)"
	@echo "    make fmt                   gofmt -w ."
	@echo "    make tidy                  go mod tidy"
	@echo ""
	@echo "  Containers:"
	@echo "    make docker-build          Build all three Docker images"
	@echo "    make docker-push           Push to ghcr.io (requires login)"

build: controlplane mcp-gateway webhook-receiver

controlplane:
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o bin/leloir-controlplane ./cmd/leloir-controlplane

mcp-gateway:
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o bin/leloir-mcp-gateway ./cmd/leloir-mcp-gateway

webhook-receiver:
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o bin/leloir-webhook-receiver ./cmd/leloir-webhook-receiver

test:
	go test ./... -v

test-short:
	go test ./... -short -v

test-race:
	go test ./... -race -v

vet:
	go vet ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

clean:
	rm -rf bin/

docker-build:
	docker build -t ghcr.io/leloir/controlplane:$(VERSION) -f Dockerfile.controlplane .
	docker build -t ghcr.io/leloir/mcp-gateway:$(VERSION) -f Dockerfile.mcp-gateway .
	docker build -t ghcr.io/leloir/webhook-receiver:$(VERSION) -f Dockerfile.webhook-receiver .

docker-push: docker-build
	docker push ghcr.io/leloir/controlplane:$(VERSION)
	docker push ghcr.io/leloir/mcp-gateway:$(VERSION)
	docker push ghcr.io/leloir/webhook-receiver:$(VERSION)
