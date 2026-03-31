.PHONY: all build test clean run fmt fmt-check vet release-linux-amd64

# The name of the resulting executable
BINARY_NAME=helm-manifest-renderer
DIST_DIR=dist
VERSION?=dev

all: fmt vet test build

build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) ./cmd/...

test:
	@echo "Running tests..."
	go test -v ./...

fmt:
	@echo "Formatting code..."
	go fmt ./...

fmt-check:
	@echo "Checking code formatting..."
	@test -z "$$(gofmt -l .)" || (echo "The following files need formatting:" && gofmt -l . && exit 1)

vet:
	@echo "Running go vet..."
	go vet ./...

clean:
	@echo "Cleaning up..."
	go clean
	rm -f $(BINARY_NAME)
	rm -rf tmp generated-manifests $(DIST_DIR)

run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

release-linux-amd64:
	@echo "Building release artifact for linux/amd64 ($(VERSION))..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(DIST_DIR)/$(BINARY_NAME) ./cmd/...
	tar -C $(DIST_DIR) -czf $(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_linux_amd64.tar.gz $(BINARY_NAME)
	sha256sum $(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_linux_amd64.tar.gz > $(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_checksums.txt
	rm -f $(DIST_DIR)/$(BINARY_NAME)
