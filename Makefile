.PHONY: build test lint fmt vet clean docker help

BINARY_NAME=changate
BUILD_DIR=dist
GO=go
GOFLAGS=-v

help: ## Show this help message
	@echo "Changate Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the binary
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

test: ## Run all tests
	$(GO) test -v -race -cover ./...

test-coverage: ## Run tests with coverage report
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint: ## Run golangci-lint
	golangci-lint run ./...

fmt: ## Format code (gofmt + goimports)
	gofmt -w .
	@which goimports > /dev/null && goimports -w . || true

vet: ## Run go vet
	$(GO) vet ./...

clean: ## Clean build artifacts
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

docker-build: ## Build Docker image
	docker build -t atompi/changate .

docker-run: ## Run Docker container
	docker run -p 8080:8080 atompi/changate

docker-compose-up: ## Run with docker-compose
	docker-compose -f docker/docker-compose.yaml up -d

docker-compose-down: ## Stop docker-compose
	docker-compose -f docker/docker-compose.yaml down

gofmt-check: ## Check if code is formatted (for CI)
	@files=$$($(GO) fmt ./... 2>&1); \
	if [ -n "$$files" ]; then \
		echo "Unformatted files:"; \
		echo "$$files"; \
		exit 1; \
	fi

check: fmt-check vet lint test ## Run all checks (format, vet, lint, test)