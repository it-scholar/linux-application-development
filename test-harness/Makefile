.PHONY: all build clean test install deps

BINARY_NAME=test-harness
BUILD_DIR=bin
CMD_DIR=cmd/harness

all: deps build

deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

build: deps
	@echo "Building $(BINARY_NAME)..."
	mkdir -p $(BUILD_DIR)
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

build-debug:
	@echo "Building $(BINARY_NAME) with debug symbols..."
	mkdir -p $(BUILD_DIR)
	go build -gcflags="all=-N -l" -o $(BUILD_DIR)/$(BINARY_NAME)-debug ./$(CMD_DIR)

clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	go clean -cache

test:
	@echo "Running tests..."
	go test -v ./...

test-short:
	@echo "Running short tests..."
	go test -short ./...

install: build
	@echo "Installing..."
	cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/

run: build
	./$(BUILD_DIR)/$(BINARY_NAME) --help

# Development helpers
fmt:
	@echo "Formatting code..."
	go fmt ./...

vet:
	@echo "Running go vet..."
	go vet ./...

lint: fmt vet
	@echo "Linting complete"

# Data retrieval
retrieve:
	./$(BUILD_DIR)/$(BINARY_NAME) retrieve --limit 1000 --country US

# Docker
docker-build:
	docker build -t weather-station/test-harness:latest .

docker-run:
	docker run --rm -v $$(pwd)/services:/services weather-station/test-harness:latest

.DEFAULT_GOAL := build
