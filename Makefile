.PHONY: build test fmt

VERSION ?= dev

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/dh ./cmd/dh

test:
	go test ./...

fmt:
	gofmt -w cmd internal pkg
