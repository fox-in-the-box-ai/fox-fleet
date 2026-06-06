.PHONY: build test lint clean conformance

build:
	go build ./cmd/fox-control

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/ dist/

conformance:
	go test ./conformance/...
