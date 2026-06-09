# ADR 0015: Binary Integration Tests Deferred

**Status:** Accepted
**Date:** 2026-06-09

## Context

The v1.4.0 grand audit (C-01, C-02, H-02, H-04) identified that several config-to-runtime wiring paths lacked test coverage. The data-plane server, AutoRestart config, RateLimit config, and Qdrant health were all defined but never integration-tested at the binary level — unit tests covered individual packages but not the wiring in `cmd/fox-control/main.go`.

Binary-level integration tests for `fox-control serve` require a running Docker daemon, a Qdrant instance, and an embedding service. This infrastructure is not available in the current CI environment (GitHub Actions hosted runners) without significant setup.

## Decision

Ship v1.4.1 with the wiring fixes verified by:
1. Successful `go build ./...` (compilation proves type-level wiring)
2. `go vet ./...` (static analysis)
3. Package-level unit tests (`go test ./...`) covering each component in isolation
4. Manual review of the `main.go` wiring paths

Defer full binary integration tests (start `fox-control serve`, hit `/healthz`, verify data-plane routes, test SSE with real config) to a future ticket. The test harness should use `testcontainers-go` or a similar approach to stand up Docker + Qdrant in CI.

## Consequences

- Config-wiring regressions at the binary level remain possible until integration tests land.
- The risk is low: the wiring code is straightforward struct plumbing with no conditional logic.
- A future PR should add a `TestBinaryServe` integration test gated behind a build tag (e.g. `//go:build integration`).
