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

## Standing discipline rules

These apply to every PR and release. They exist because each was learned from a real incident.

### Rule 6 — Secret hygiene

Secrets never appear in chat output, command stdout, transcripts, or unstructured files. Generated secrets pipe to 600-permission files at known paths. Reading uses file references, never echoed variable contents.

### Rule 7 — Real-browser conformance

Every change that touches a web surface (login pages, admin panel, SSE endpoints) must be verified in a real browser, not just `curl`. Browser rendering, CSP, cookie behavior, and CORS are not testable from the command line.

### Rule 8 — Fresh-deploy conformance

Release-tagged builds must pass a fresh-deploy test: pull the image, run with empty volumes, provision an instance, verify health. This catches first-run failures that incremental testing misses.

---

## Security

See [SECURITY.md](https://github.com/fox-in-the-box-ai/fox-fleet/blob/main/SECURITY.md) for vulnerability reporting policy.
