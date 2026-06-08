# Apache Fleet Base — Release Roadmap v1.1.0–v1.4.0

> Post-v1.0.1 forward plan. 50 consensus tickets across 4 minor releases, ~9 weeks total.
> Source: 6-agent panel deliberation (3 Architects + 3 SWEs), adversarial review applied.
> Canonical ticket specifications: [APACHE_BACKLOG_v1.md](APACHE_BACKLOG_v1.md)

---

## Release Sequence

| Version | Theme | Tickets | Est. Weeks | Cumulative |
|---------|-------|---------|------------|------------|
| v1.1.0 | Security Hardening & Operator Robustness | 17 | 2 | 2 |
| v1.2.0 | Operator Integrations & Observability | 11 | 2 | 4 |
| v1.3.0 | Data Plane Quality & RAG Reliability | 14 | 2.5 | 6.5 |
| v1.4.0 | Contributor Experience & Panel Completeness | 8 | 2.5 | 9 |

---

## v1.1.0 — Security Hardening & Operator Robustness

**Narrative:** Ships all 11 security audit findings (H1–H4, M1/M4/M5, I1–I5) plus critical operator reliability fixes: port reclamation, persistent event log, graceful shutdown, duplicate-instance race condition, and database migration versioning. This release makes Fox Fleet production-trustworthy.

**Tickets (17):**

| ID | Title | Size |
|----|-------|------|
| SEC-01 | Fix hermes.env file permissions (0644 → 0600) | XS |
| SEC-02 | Harden REST connector against SSRF | S |
| SEC-03 | Restrict file connector to allowed directory | S |
| SEC-04 | Add HTTP server WriteTimeout | S |
| SEC-05 | Validate InstanceConfig.Env against blocklist | XS |
| SEC-06 | Enforce admin_secret minimum length + generate-secret command | XS |
| SEC-07 | Validate instance ID format in handleDetail/handleDestroy | XS |
| SEC-08 | Add digest-only image reference warning | XS |
| SEC-09 | Add security headers to embedded SPA | XS |
| SEC-10 | Replace fmt.Fprintf YAML with yaml.v3 Marshal | XS |
| SEC-11 | URL-escape Qdrant collection names in API paths | XS |
| OPS-01 | Fix port reclamation on instance destroy | XS |
| OPS-02 | Persistent event log (SQLite-backed) | M |
| PLAT-02 | Graceful shutdown with in-flight request draining | S |
| PLAT-04 | Registry database migration versioning system | S |
| PANEL-03 | Fix handleCreate race condition (false 201) | S |
| DP-05 | Data plane WriteTimeout and request body limits | XS |

**Velocity:** 10 XS + 5 S + 2 M = ~2 weeks. Aggressive but feasible — XS tickets are 1–2 hour fixes with tests.

**Release notes draft:**

### Security
- Fixed file permissions for secret-containing files (hermes.env, tools.json) from 0644 to 0600 (SEC-01)
- Added SSRF protection to REST ingestion connector: blocks RFC-1918, link-local, loopback, and cloud metadata endpoints; custom net.Dialer validates resolved IP on every connection to prevent DNS rebinding (SEC-02)
- Restricted file connector to configurable allowed directory with `allowedDir + filepath.Separator` prefix check and symlink traversal protection (SEC-03)
- Added WriteTimeout to HTTP server with ResponseController.SetWriteDeadline in SSE handler (SEC-04)
- Validated InstanceConfig.Env keys against security-sensitive blocklist (PATH, LD_PRELOAD, FOX_*, HERMES_*) (SEC-05)
- Enforced minimum 16-character length for admin_secret; added `fox-control generate-secret` command (SEC-06)
- Added instance ID format validation in handleDetail and handleDestroy (SEC-07)
- Added digest-only image reference warning for supply-chain hardening (SEC-08)
- Added security headers (CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy) to embedded SPA (SEC-09)
- Replaced fmt.Fprintf YAML construction with yaml.v3 Marshal for correct escaping (SEC-10)
- URL-escaped Qdrant collection names in API paths (SEC-11)

### Added
- Persistent event log backed by SQLite with configurable retention (default 7 days) and `fox-control events` CLI command (OPS-02)
- Graceful shutdown with in-flight request draining and configurable drain timeout (PLAT-02)
- Registry database migration versioning system with forward-only numbered migrations (PLAT-04)
- Data plane WriteTimeout (300s) and request body limits via MaxBytesReader (DP-05)

### Fixed
- Port reclamation on instance destroy: freed ports are immediately reusable without restart (OPS-01)
- Race condition in handleCreate: concurrent requests for the same instance ID now produce exactly one success and 409 Conflict for others (PANEL-03)

---

## v1.2.0 — Operator Integrations & Observability

**Narrative:** Adds the monitoring and integration surface operators need for production deployments: Prometheus metrics, structured JSON logging, webhook event forwarding, TLS termination, backup/restore tooling, diagnostics command, and rate limiting. After this release, Fox Fleet integrates cleanly into standard operator toolchains (Prometheus/Grafana, ELK/Loki, webhook-driven alerting).

**Tickets (11):**

| ID | Title | Size |
|----|-------|------|
| OPS-03 | Prometheus metrics endpoint (/metrics) | S |
| OPS-04 | Built-in TLS termination | S |
| OPS-05 | SQLite backup/restore CLI commands (VACUUM INTO) | S |
| OPS-06 | Diagnostics command (8+ health checks) | M |
| INT-01 | Webhook event forwarding with HMAC-SHA256 signing | M |
| INT-02 | Structured JSON logging | S |
| POLISH-01 | Token bucket rate limiting on API endpoints | S |
| POLISH-02 | CLI --output flag (table/json/quiet) | S |
| PERF-01 | ValidQueryToken hash-then-lookup optimization | XS |
| REL-01 | govulncheck in CI pipeline | S |
| REL-02 | Automated dependency updates (Dependabot) | S |

**Dependencies on v1.1.0:** INT-01 → OPS-02 (event log as webhook source).

**Velocity:** 1 XS + 8 S + 2 M = ~2 weeks.

**Release notes draft:**

### Added
- Prometheus metrics endpoint at /metrics with instance gauge, health gauge, provision duration histogram, API request histogram, and SSE connections gauge (OPS-03)
- Built-in TLS termination via `tls.cert_file` and `tls.key_file` configuration (OPS-04)
- SQLite backup/restore CLI commands using VACUUM INTO for consistent snapshots (OPS-05)
- Diagnostics command (`fox-control diagnostics`) running 8+ health checks: config validity, Docker socket, registry integrity, Qdrant reachability, embedding service, disk space, port availability (OPS-06)
- Webhook forwarding: POST-on-event with HMAC-SHA256 signing, configurable event filters, 5s timeout, 10/s rate limit per endpoint (INT-01)
- Structured JSON logging via `control.log_format = json` with configurable log level (INT-02)
- Token bucket rate limiting on API endpoints: 100 req/min global, 10 provisions/min; /healthz and /metrics exempt (POLISH-01)
- CLI output formatting: --output flag supporting table, json, and quiet modes (POLISH-02)
- govulncheck in CI pipeline for continuous vulnerability scanning (REL-01)
- Automated dependency update workflow via Dependabot (REL-02)

### Changed
- ValidQueryToken optimized from O(N) Go-side iteration to indexed hash-then-lookup with constant-time final comparison (PERF-01)

---

## v1.3.0 — Data Plane Quality & RAG Reliability

**Narrative:** Transforms the data plane from a working prototype into a production-grade knowledge pipeline: embedding retry with backoff, batched upserts, incremental ingestion with content hashing, per-operation Qdrant timeouts, Qdrant health monitoring, source deletion cascade, dimension validation, score threshold filtering, and embedding request batching. After this release, the RAG pipeline handles real-world failure modes gracefully and operates efficiently at scale.

**Tickets (14):**

| ID | Title | Size |
|----|-------|------|
| DP-01 | Embedding client retry with exponential backoff | S |
| DP-02 | Per-operation Qdrant client timeouts | S |
| DP-03 | Qdrant health monitoring integrated into health poller | S |
| DP-04 | Incremental source re-ingestion with content hashing | M |
| DP-06 | Qdrant upsert batching for large ingestions | S |
| DP-07 | Source deletion cascade (remove Qdrant points) | S |
| DP-08 | Embedding dimension validation on startup | S |
| DP-09 | Query result score threshold filtering | S |
| PERF-02 | Embedding request batching in ingestion connectors | M |
| PLAT-01 | Runtime validation of skillset-declared tools | S |
| PLAT-03 | Health-check-based auto-restart for unhealthy instances | M |
| CONF-01 | Data plane round-trip conformance checks | M |
| CONF-02 | Security-focused conformance checks | S |
| REL-03 | Release health monitoring workflow | M |

**Dependencies on prior releases:** DP-01 ← DP-04, PERF-02; DP-02 ← DP-03; SEC-09 (v1.1.0) ← CONF-02; REL-01 (v1.2.0) ← REL-03.

**Velocity:** 1 XS + 8 S + 5 M = ~2.5 weeks.

**Release notes draft:**

### Added
- Retry with exponential backoff and jitter for embedding client: retries on 429/5xx, respects context cancellation (DP-01)
- Per-operation Qdrant client timeouts: 5s health, 30s search, 60s upsert, 30s collection ops (DP-02)
- Qdrant health monitoring integrated into fox-control health poller with event emission on state transitions (DP-03)
- Incremental source re-ingestion with SHA-256 content hashing: skips unchanged documents, re-embeds modified ones, removes deleted points (DP-04)
- Qdrant upsert batching for large ingestions with configurable batch size (default 100 points) (DP-06)
- Source deletion cascade: DELETE /admin/sources/{id} removes all associated Qdrant points (DP-07)
- Embedding dimension validation on startup: detects config/model dimension mismatch before data corruption (DP-08)
- Query result score threshold filtering via optional min_score parameter (DP-09)
- Embedding request batching in ingestion connectors with configurable batch size (default 256 chunks) (PERF-02)
- Runtime validation of skillset-declared tools against instance image (PLAT-01)
- Health-check-based auto-restart for unhealthy instances: configurable threshold, cooldown, per-instance opt-out (default off) (PLAT-03)
- Data plane round-trip conformance checks (CONF-01)
- Security-focused conformance checks (CONF-02)
- Release health monitoring workflow (REL-03)

---

## v1.4.0 — Contributor Experience & Panel Completeness

**Narrative:** Rounds out the user-facing panel with health history, resource usage, skillset selection, and i18n contribution workflow. Delivers comprehensive documentation: operator handbook, developer guide, API reference, and example deployments. After this release, both operators and contributors have the documentation and tooling they need to adopt and extend Fox Fleet independently.

**Tickets (8):**

| ID | Title | Size |
|----|-------|------|
| PANEL-01 | Instance health history timeline and uptime indicator | S |
| PANEL-02 | Per-instance resource usage display (CPU/memory/network) | S |
| PANEL-04 | i18n contribution workflow | S |
| PANEL-05 | Instance create form with skillset dropdown | S |
| DOC-01 | Operator handbook | L |
| DOC-02 | Developer and contributor handbook | M |
| DOC-03 | API reference (OpenAPI 3.0) | M |
| DOC-04 | Example deployment configurations | S |

**Dependencies on prior releases:** PANEL-01 → OPS-02 (v1.1.0); DOC-01 → OPS-02 (v1.1.0), OPS-03, OPS-05 (v1.2.0).

**Velocity:** 4 S + 2 M + 1 L + 1 S = ~2.5 weeks.

**Release notes draft:**

### Added
- Instance health history timeline and uptime indicator in panel detail view, backed by persistent event log (PANEL-01)
- Per-instance resource usage display (CPU, memory, network I/O) via Docker container stats API (PANEL-02)
- i18n contribution workflow: extracted translation strings, contribution guide, additional language support (PANEL-04)
- Instance create form with skillset dropdown and role selection in provision modal (PANEL-05)
- Operator handbook: production checklist, backup/restore guide, monitoring setup with Prometheus scrape config examples, capacity planning, secret rotation, 15+ troubleshooting scenarios (DOC-01)
- Developer and contributor handbook: architecture overview with diagrams, package layout, plugin and connector development guides, ADR authoring guide (DOC-02)
- API reference documentation for panel and data plane APIs with OpenAPI 3.0 spec and tested curl examples (DOC-03)
- Example deployment configurations: minimal single-binary, production Docker Compose with TLS, air-gapped with pre-pulled images (DOC-04)

---

## Sequencing Rationale

**Security first, then integrations, then data quality, then docs.**

1. **v1.1.0** — All 11 security findings must ship ASAP. Bundled with zero-dependency robustness fixes (port reclamation, graceful shutdown, migration versioning) and OPS-02 (persistent event log) because it's a dependency for 3 downstream tickets across 2 later releases.

2. **v1.2.0** — Once the platform is secure, operators need toolchain integration. Prometheus, structured logging, webhooks, TLS, backup, and diagnostics form a coherent "production operations" story. REL-01/REL-02 (CI security) land before the data plane expansion in v1.3.0.

3. **v1.3.0** — Data plane tickets form a tightly coupled set with clear internal dependencies (DP-01 → DP-04, PERF-02; DP-02 → DP-03). Splitting them across releases would ship incomplete resilience. PLAT-01/PLAT-03 and CONF-01/CONF-02 verify properties shipped in v1.1.0–v1.3.0.

4. **v1.4.0** — Documentation and panel polish depend on the features they document and display. DOC-01 depends on OPS-02, OPS-03, OPS-05 from v1.1.0–v1.2.0. PANEL-01 depends on OPS-02 from v1.1.0.

**Dependency chain verification:** All dependency pairs are satisfied — no ticket ships before its dependencies. See the dependency graph in [APACHE_BACKLOG_v1.md](APACHE_BACKLOG_v1.md).

**SemVer:** All four releases are minor (v1.1.0–v1.4.0) — new features and fixes, no breaking API changes. SEC-08 deliberately chose warn-not-reject to avoid a breaking change.

**Total: 50 tickets, 4 releases, ~9 weeks.** The 9-week estimate includes buffer for integration testing between releases. If velocity holds, v1.3.0 and v1.4.0 could each compress to 2 weeks (8 weeks total).
