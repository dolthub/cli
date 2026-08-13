.PHONY: build test lint fmt

VERSION ?= dev

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/dh ./cmd/dh

test:
	go test ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w cmd internal pkg
