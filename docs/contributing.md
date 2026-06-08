# Contributing

Branch from `main`, one logical change per PR, all checks green, squash-merge. Commit messages in imperative present tense, referencing ticket IDs.

For the full contributing guide, see [`CONTRIBUTING.md`](https://github.com/fox-in-the-box-ai/fox-fleet/blob/main/CONTRIBUTING.md) in the repository root.

---

## Development setup

### Prerequisites

- Go 1.25+
- Docker (for integration tests and the Docker plugin)
- [golangci-lint](https://golangci-lint.run/) (for `make lint`)

### Quality gate

```bash
make lint          # golangci-lint run
make test          # go test -race -shuffle=on -count=1 ./...
make build         # go build with ldflags
make conformance   # runtime + plugin conformance suites
```

All four commands must pass before opening a PR.

### Running tests

```bash
# All tests
make test

# Specific package
go test ./internal/provisioner/...
go test ./internal/registry/...
go test ./panel/api/...
```

Tests use temporary directories and in-memory SQLite — no Docker daemon required for unit tests.

---

## Security

See [SECURITY.md](https://github.com/fox-in-the-box-ai/fox-fleet/blob/main/SECURITY.md) for vulnerability reporting policy.
