.PHONY: all build test clean run fmt vet

# The name of the resulting executable
BINARY_NAME=helm-manifest-renderer

all: fmt vet test build

build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) ./cmd/helm-manifest-renderer

test:
	@echo "Running tests..."
	go test -v ./...

fmt:
	@echo "Formatting code..."
	go fmt ./...

vet:
	@echo "Running go vet..."
	go vet ./...

clean:
	@echo "Cleaning up..."
	go clean
	rm -f $(BINARY_NAME)
	rm -rf tmp generated-manifests

run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)
