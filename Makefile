# Makefile for swagger-docs-mcp Go implementation

.PHONY: build test clean install dev run-dev fmt vet deps check help

# Build variables
BINARY_NAME=swagger-docs-mcp
MAJOR=1
MINOR=0
PATCH=0
PRERELEASE=
COMMIT_COUNT=$(shell git rev-list --count HEAD 2>/dev/null || echo "0")
BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
COMMIT_HASH=$(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_USER=$(shell whoami 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-s -w \
	-X swagger-docs-mcp/pkg/version.Major=$(MAJOR) \
	-X swagger-docs-mcp/pkg/version.Minor=$(COMMIT_COUNT) \
	-X swagger-docs-mcp/pkg/version.Patch=$(PATCH) \
	-X swagger-docs-mcp/pkg/version.PreRelease=$(PRERELEASE) \
	-X swagger-docs-mcp/pkg/version.CommitCount=$(COMMIT_COUNT) \
	-X swagger-docs-mcp/pkg/version.BuildDate=$(BUILD_TIME) \
	-X swagger-docs-mcp/pkg/version.CommitHash=$(COMMIT_HASH) \
	-X swagger-docs-mcp/pkg/version.BuildUser=$(BUILD_USER)"

# Default target
all: check build

# Help target
help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

deps: ## Download dependencies
	go mod download
	go mod tidy

fmt: ## Format Go code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

check: fmt vet ## Run all checks (fmt, vet)
	@echo "All checks passed"

test: ## Run tests
	go test ./...

test-cover: ## Run tests with coverage
	go test -cover ./...

test-verbose: ## Run tests with verbose output
	go test -v ./...

dev: ## Run in development mode with debug logging
	go run main.go --debug --log-level debug

run-dev: ## Run development server with example config
	go run main.go --swagger-path ./swagger_docs --debug

##@ Building

build: ## Build the binary
	go build $(LDFLAGS) -o $(BINARY_NAME)

build-race: ## Build with race detection
	go build -race $(LDFLAGS) -o $(BINARY_NAME)

build-all: ## Build for all platforms
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe

install: build ## Install the binary to GOPATH/bin
	cp $(BINARY_NAME) $(GOPATH)/bin/

##@ Docker

docker-build: ## Build Docker image with version information
	./aws/deploy.sh build

docker-run: ## Run in Docker container
	./docker-run.sh

docker-run-detached: ## Run in Docker container (detached)
	./docker-run.sh -d

docker-compose-up: ## Start with docker-compose
	docker-compose up

docker-compose-down: ## Stop docker-compose services
	docker-compose down

docker-compose-build: ## Build with docker-compose
	docker-compose build

docker-logs: ## Show container logs
	docker logs -f swagger-mcp-server

docker-shell: ## Access container shell
	docker exec -it swagger-mcp-server sh

docker-clean: ## Clean up Docker resources
	docker system prune -f
	docker image prune -f

##@ AWS Deployment

aws-ecr: ## Create AWS ECR repository
	./aws/deploy.sh ecr

aws-build: ## Build and push Docker image to AWS ECR
	./aws/deploy.sh build

aws-stack: ## Deploy CloudFormation stack
	./aws/deploy.sh stack

aws-lambda: ## Deploy Lambda MCP proxy
	./aws/deploy.sh lambda

aws-deploy: ## Full AWS deployment (all steps)
	./aws/deploy.sh all

##@ Utilities

clean: ## Clean build artifacts
	go clean
	rm -f $(BINARY_NAME)*

config: build ## Show current configuration
	./$(BINARY_NAME) config

version: build ## Show version information
	./$(BINARY_NAME) version

example: build ## Run with example configuration
	./$(BINARY_NAME) --swagger-path ./swagger_docs/v1 --swagger-path ./swagger_docs/v2 --debug

benchmark: ## Run benchmarks
	go test -bench=. ./...

size: build ## Show binary size
	@echo "Binary size:"
	@ls -lh $(BINARY_NAME) | awk '{print $$5 "\t" $$9}'

##@ Testing different scenarios

test-paths: build ## Test with swagger paths
	./$(BINARY_NAME) --swagger-path ./swagger_docs/v1 --debug --log-level debug

test-urls: build ## Test with swagger URLs (example)
	./$(BINARY_NAME) --swagger-url https://petstore.swagger.io/v2/swagger.json --debug

test-config: build ## Test with config file
	./$(BINARY_NAME) --config swagger-mcp.config.example.json --debug

test-filters: build ## Test with package filters
	./$(BINARY_NAME) --swagger-path ./swagger_docs --package-ids weather,alerts --debug

test-twc: build ## Test with TWC filters
	./$(BINARY_NAME) --swagger-path ./swagger_docs --twc-domains forecast,current --debug

##@ Performance

profile-cpu: ## Run with CPU profiling
	go build -o $(BINARY_NAME)-profile
	./$(BINARY_NAME)-profile --swagger-path ./swagger_docs &
	sleep 30
	kill %%

profile-mem: ## Run with memory profiling
	go build -o $(BINARY_NAME)-profile
	./$(BINARY_NAME)-profile --swagger-path ./swagger_docs &
	sleep 30
	kill %%

##@ Maintenance

update-deps: ## Update dependencies
	go get -u ./...
	go mod tidy

security: ## Run security checks
	@which gosec > /dev/null || (echo "Installing gosec..." && go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest)
	gosec ./...

lint: ## Run golangci-lint
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

##@ Documentation

docs: ## Generate documentation
	@echo "Generating documentation..."
	@echo "API documentation available at: https://pkg.go.dev/$(shell go list -m)"

readme: ## Display README
	@cat README-GO.md

##@ CI/CD

ci: deps check test build ## Run full CI pipeline
	@echo "CI pipeline completed successfully"

release: clean check test build-all ## Prepare release artifacts
	@echo "Release artifacts built:"
	@ls -la $(BINARY_NAME)*

# Include local Makefile if it exists
-include Makefile.local