# ADR 0015: Binary Integration Tests Required

**Status:** Accepted
**Date:** 2026-06-09

## Context

The v1.4.0 grand audit found four phantom features — code paths that compiled but were never exercised at runtime:

- **C-01:** Data-plane HTTP server was defined but never started in the binary
- **C-02:** AutoRestart config was parsed but never forwarded to the HealthPoller
- **H-02:** RateLimit config was parsed but never forwarded to the API server
- **H-04:** Qdrant health was monitored but never surfaced in `/healthz`

All four passed unit tests because each package was tested in isolation. The wiring in `cmd/fox-control/main.go` — where config flows into `Deps` and `Deps` flows into servers — had zero test coverage. The bugs were invisible until an adversarial audit read the code end-to-end.

## Decision

Any PR that adds wiring between packages — a new server, daemon, long-running component, or feature that depends on config flowing through `Deps` into a runtime component — must include a binary-level integration test that exercises the published binary.

**Minimum bar for a wiring integration test:**

1. Build the `fox-control` binary
2. Start it with a test config that enables the feature under test
3. Hit the relevant endpoint or observe the relevant behavior
4. Assert the feature is actually active, not just compiled in

**Implementation approach:** Use `//go:build integration` build tags so these tests don't run in the default `go test ./...` pass (they need Docker, network, sometimes Qdrant). CI runs them in the `conformance` job which already has Docker available.

**PR checklist enforcement:** The PR template includes a checkbox: "If this PR adds wiring between packages, the test suite includes a binary-level integration test."

## Consequences

- PRs that add new wiring paths without integration tests are blocked at review.
- The integration test suite grows alongside the feature set, catching phantom features before they ship.
- Developers must ensure Docker is available when running integration tests locally (`go test -tags integration ./...`).
- The lesson from v1.4.1's C-01 and C-02 is codified: compilation is not verification.
