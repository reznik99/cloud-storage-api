#!/bin/bash

VERSION=$(git describe --tags)
GOOS=linux 
GOARCH=amd64 
CGO_ENABLED=0 
go build -v -ldflags="-X 'main.Version=${VERSION}'" -o dist/storage-api .