.PHONY: help test test-short test-race test-coverage build vet lint tidy clean fmt

help:
	@echo "Leloir SDK — make targets:"
	@echo "  make test          - run all tests"
	@echo "  make test-short    - run tests that don't need network"
	@echo "  make test-race     - run tests with race detector"
	@echo "  make test-coverage - run tests with coverage report"
	@echo "  make build         - build all packages (verifies compilation)"
	@echo "  make vet           - run go vet"
	@echo "  make lint          - run golangci-lint (requires install)"
	@echo "  make tidy          - run go mod tidy"
	@echo "  make fmt           - run gofmt -w ."
	@echo "  make clean         - remove build artifacts"

test:
	go test ./... -v

test-short:
	go test ./... -short -v

test-race:
	go test ./... -race -v

test-coverage:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

build:
	go build ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

fmt:
	gofmt -w .

clean:
	rm -f coverage.out coverage.html
