VERSION := $(shell git describe --tags)
LDFLAGS := -X 'main.Version=$(VERSION)'

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -ldflags="$(LDFLAGS)" -o dist/storage-api .
