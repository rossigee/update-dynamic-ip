.PHONY: test lint build clean install-golangci-lint

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=update-dynamic-ip

# golangci-lint version
GOLANGCI_LINT_VERSION=v2.4.0

# Build the binary
build:
	$(GOBUILD) -o $(BINARY_NAME) -v

# Run tests
test:
	$(GOTEST) -v ./...

# Run linter with golangci-lint v2.4.0
lint: install-golangci-lint
	./bin/golangci-lint run

# Install golangci-lint v2.4.0 locally
install-golangci-lint:
	@if [ ! -f ./bin/golangci-lint ]; then \
		echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."; \
		mkdir -p ./bin; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ./bin $(GOLANGCI_LINT_VERSION); \
	fi

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf ./bin

# Tidy dependencies
tidy:
	$(GOMOD) tidy

# Download dependencies
deps:
	$(GOMOD) download

# Run all checks
check: tidy test lint

# Default target
all: clean deps build test lint