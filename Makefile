.PHONY: build test lint clean conformance

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  = -X main.buildVersion=$(VERSION) -X main.buildCommit=$(COMMIT) -X main.buildDate=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o fox-control ./cmd/fox-control

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/ dist/ fox-control

conformance:
	go test ./conformance/...
