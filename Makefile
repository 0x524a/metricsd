BINARY_NAME=metricsd
BUILD_DIR=bin
CMD_DIR=cmd/metricsd

.PHONY: all build run clean test deps help

all: build

## build: Build the application binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)/main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

## run: Run the application with default config
run:
	@echo "Running $(BINARY_NAME)..."
	go run $(CMD_DIR)/main.go

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f config.json
	@echo "Clean complete"

## test: Run tests
test:
	@echo "Running tests..."
	go test -v ./...

## deps: Download and tidy dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies updated"

## fmt: Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

## lint: Run golangci-lint (requires golangci-lint to be installed)
lint:
	@echo "Running linter..."
	golangci-lint run

## install: Install the binary to GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	go install $(CMD_DIR)/main.go

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):latest .

## help: Display this help message
help:
	@echo "Available targets:"
	@grep -E '^## ' Makefile | sed 's/^## //'
