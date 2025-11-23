.PHONY: help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || git describe --tags 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

LDFLAGS := -X 'github.com/LoriKarikari/draftdeploy/internal/version.version=$(VERSION)' \
           -X 'github.com/LoriKarikari/draftdeploy/internal/version.commit=$(COMMIT)'

.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o bin/draftdeploy cmd/draftdeploy/main.go

.PHONY: install
install: build
	go install ./cmd/draftdeploy

.PHONY: run
run:
	go run cmd/draftdeploy/main.go

.PHONY: test
test:
	go test -v -race -cover ./...

.PHONY: test-coverage
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: lint
lint:
	@if ! which golangci-lint > /dev/null; then \
		echo "golangci-lint not found, installing..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	golangci-lint run ./...

.PHONY: gosec
gosec:
	@if ! which gosec > /dev/null; then \
		echo "gosec not found, installing..."; \
		go install github.com/securego/gosec/v2/cmd/gosec@latest; \
	fi
	gosec ./...

.PHONY: fmt
fmt:
	go fmt ./...
	gofmt -s -w .

.PHONY: vet
vet:
	go vet ./...

.PHONY: mod
mod:
	go mod download
	go mod tidy

.PHONY: clean
clean:
	rm -rf bin/ dist/ coverage.* *.test

.PHONY: check
check: fmt vet lint gosec test

.PHONY: ci
ci: mod check build

.PHONY: docker-build
docker-build:
	docker build -t draftdeploy:latest .

.PHONY: docker-run
docker-run:
	docker run --rm -it draftdeploy:latest

.PHONY: release
release:
	@echo "Building release binaries..."
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/draftdeploy-linux-amd64 cmd/draftdeploy/main.go
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/draftdeploy-linux-arm64 cmd/draftdeploy/main.go
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/draftdeploy-darwin-amd64 cmd/draftdeploy/main.go
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/draftdeploy-darwin-arm64 cmd/draftdeploy/main.go
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/draftdeploy-windows-amd64.exe cmd/draftdeploy/main.go
	@echo "Release binaries built in dist/"

.PHONY: dev
dev:
	@if ! which air > /dev/null; then \
		echo "air not found, installing..."; \
		go install github.com/cosmtrek/air@latest; \
	fi
	air

.DEFAULT_GOAL := help