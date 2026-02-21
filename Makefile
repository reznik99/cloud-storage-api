VERSION := $(shell git describe --tags)
LDFLAGS := -X 'main.Version=$(VERSION)'

all: lint test build

build:
	@echo "==> Building $(VERSION) for linux/amd64..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -ldflags="$(LDFLAGS)" -o dist/storage-api .

test:
	@echo "==> Running tests..."
	@go test ./...

lint:
	@echo "==> Linting..."
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run ./...
