# ADR-0018: Per-Package Coverage Gates

**Date:** 2026-06-09
**Status:** Accepted
**Deciders:** Dennis Vorobyov

## Context

The CI coverage gate uses a single global threshold (45%) applied to the
aggregate of all non-conformance packages. This masks per-package variance:
a critical-path package at 9% is hidden by a utility package at 96%.

Per-package coverage as of v1.4.2 (conformance packages excluded):

| Package | Coverage | Classification | Gate |
|---------|----------|----------------|------|
| `cmd/fox-control` | 32.7% | CLI entrypoint | 30% |
| `data-plane/chunker` | 95.5% | Data plane | 70% |
| `data-plane/embedding` | 86.2% | Data plane | 70% |
| `data-plane/ingestion/file` | 73.1% | Data plane | 70% |
| `data-plane/ingestion/rest` | 81.3% | Data plane | 70% |
| `data-plane/qdrant` | 53.8% | Data plane (infra) | 50% |
| `data-plane/server` | 59.4% | Data plane | 55% |
| `data-plane/source` | 64.0% | Data plane | 60% |
| `internal/config` | 90.2% | Critical path | 70% |
| `internal/events` | 29.4% | Infrastructure | 25% |
| `internal/output` | 0.0% | CLI formatting | 0% |
| `internal/provisioner` | 75.5% | Critical path | 70% |
| `internal/registry` | 45.0% | Critical path | 40% |
| `internal/safedialer` | 81.2% | Security | 70% |
| `internal/sessiontoken` | 96.8% | Security | 70% |
| `panel/api` | 69.9% | Critical path | 65% |
| `plugins/docker` | 9.2% | Critical path (infra) | 5% |
| `rollout` | 84.7% | Critical path | 70% |
| `skillsets` | 66.0% | Feature | 60% |

### Classification rationale

**Critical path** — operator-facing runtime packages whose failure breaks
fleet operations (provisioning, registry, API, rollout). These carry the
highest gate expectations.

**Critical path (infra)** — critical-path packages that are structurally
difficult to unit-test because every method calls an external service
(Docker daemon, database). Coverage comes primarily from integration and
conformance tests, not unit tests.

**Security** — auth and network-safety packages. High gates; regressions
here are security vulnerabilities.

**Data plane** — organizational knowledge subsystem. Important when enabled
but optional; not on the critical path for basic fleet operations.

**Data plane (infra)** — data-plane packages with heavy external dependencies
(Qdrant client). Same structural testing constraint as `plugins/docker`.

**CLI entrypoint / CLI formatting / Infrastructure** — command wiring,
output formatting, event/webhook dispatch. Low blast radius; mostly
integration-tested through the binary.

### Per-package justifications for gates below 70%

**`plugins/docker` — 9.2%, gate 5%.**
All 9 plugin methods (`Provision`, `HealthCheck`, `Configure`, `Rollout`,
`Rollback`, `Destroy`, `Logs`, `Restart`, `Stats`) make real Docker API
calls (`ContainerCreate`, `ContainerStart`, `ContainerStop`,
`ContainerRemove`, `ImagePull`, etc.). The 4 existing tests cover pure
utility functions (`extractHostPort`, `containerNaming`, `httpProbe`,
interface compliance). Mocking the Docker client would test the mock, not
the integration — the conformance suite is the real test surface for this
package. Gate set at 5% to protect the tested utilities from regression.

**`internal/registry` — 45.0%, gate 40%.**
SQLite CRUD operations. The tested surface covers `New`, `Create`, `Get`,
`List`, `UpdateStatus`, `UpdateSkillsets`, `Delete`, `SetImage`. Untested
functions are batch operations (`ListByImage`, `CountByStatus`,
`BulkUpdateImage`, `PurgeOlderThan`, `ExportJSON`, `ImportJSON`) that are
used by CLI subcommands and the rollout orchestrator. These are tested
indirectly through `rollout` package tests (84.7%) and binary integration.
Gate set at 40% to protect the tested CRUD core.

**`panel/api` — 69.9%, gate 65%.**
0.1% below 70%. Untested handlers are `handleMetricsPage` (Prometheus
exposition) and `handleDiagnostics` (debug endpoint) — both are
operational endpoints, not user-facing. The auth middleware, all instance
CRUD handlers, SSE, skillset handlers, rate limiting, health, and query
handlers are tested. Gate set at 65% as a floor; raising to 70% is a
low-effort v1.5 item.

**`data-plane/qdrant` — 53.8%, gate 50%.**
The Qdrant client methods that hit the Qdrant HTTP/gRPC API
(`EnsureCollection`, `Upsert`, `Search`, `Delete`, `StartContainer`,
`StopContainer`) require a running Qdrant instance. Tested surface
covers config, URL generation, HTTP probe, poll health, and manager
lifecycle. Data plane conformance tests cover the integration path.

**`data-plane/server` — 59.4%, gate 55%.**
HTTP server with handler chains. Untested code is primarily the
`ListenAndServe` / `Shutdown` lifecycle and the file-upload multipart
handler. Handler logic is covered.

**`data-plane/source` — 64.0%, gate 60%.**
Source registry with SQLite persistence. Untested functions are
`Register`, `Unregister`, `ListByType` which are thin CRUD wrappers.
Core query and listing logic is covered.

**`cmd/fox-control` — 32.7%, gate 30%.**
This package is 80% Cobra command wiring (`main.go`: 695 lines,
`ops.go`: 264 lines). The wiring calls into tested packages
(`provisioner`, `registry`, `config`, `plugins/docker`). Config
parsing and validation are tested at 90%+. Binary smoke tests cover
the integration path. Unit-testing Cobra command functions produces
tests that test the framework.

**`internal/events` — 29.4%, gate 25%.**
Event bus, SQLite event store, and webhook dispatcher. The event bus
publish/subscribe is tested. The SQLite store and webhook HTTP dispatch
require integration-level setup (database, HTTP server). These are
tested indirectly through `panel/api` SSE tests.

**`internal/output` — 0.0%, gate 0%.**
78-line CLI output formatting (table, JSON, quiet modes). Pure display
logic with no business rules. No test file exists. Adding tests would
test `fmt.Fprintf` and `encoding/json.Marshal` — framework testing.

**`skillsets` — 66.0%, gate 60%.**
YAML parsing and validation. Core `Parse` and `Validate` functions are
tested. Untested code is `LoadFromDir` and `MergeManifests` which are
filesystem-dependent composition functions.

## Decision

1. **Keep the global CI gate at 45%.** This catches catastrophic regressions
   across the whole project.

2. **Document per-package gates in this ADR.** These are the floors below
   which each package must not drop. CI does not enforce per-package gates
   automatically in v1.4.2 — they are verified during release closeout
   (manual or scripted).

3. **Exclude conformance packages from coverage measurement.** Conformance
   packages (`conformance/runtime/`, `conformance/plugin/`,
   `conformance/dataplane/`) are external test infrastructure. They
   contain zero unit-testable logic. Including them adds 6 packages at
   0% that drag the global average from 54.4% to 41.0%.

4. **Raise critical-path gates in v1.5.** Target gates for v1.5:
   - `internal/registry`: 60% (add tests for batch operations)
   - `panel/api`: 70% (add tests for metrics and diagnostics handlers)
   - `plugins/docker`: no change (structurally infra-dependent)

## Consequences

### Positive

- Coverage gates are explicit and justified per package, not a single
  opaque number
- Structural testing constraints are documented — future contributors
  know why `plugins/docker` is at 9% and that the conformance suite
  covers it
- Conformance exclusion is justified and auditable

### Negative

- Per-package gates are not CI-enforced in v1.4.2 (manual verification)
- Some critical-path packages (`registry`, `panel/api`) are below the
  ideal 70% floor — accepted with documented justification and v1.5
  remediation plan
