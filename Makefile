.PHONY: build test lint fmt docs

VERSION ?= dev
DOCS_OUTPUT ?= dist/docs

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/dh ./cmd/dh

test:
	go test ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w cmd internal pkg

# Generate development reference output from the freshly built executable.
docs: build
	./bin/dh generate-docs --output "$(DOCS_OUTPUT)"
