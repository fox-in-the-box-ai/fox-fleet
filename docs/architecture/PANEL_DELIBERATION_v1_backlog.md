# Panel Deliberation Record — Apache Fleet Base Backlog v1

> 6-agent panel (3 Architects + 3 SWEs), 4 rounds: independent drafts → cross-review → synthesis → release sequencing.
> Adversarial review applied to final output. 3 blocking issues identified and resolved in the backlog.

---

## Panel Composition

| Agent | Role | Specialization | Tickets Proposed |
|-------|------|---------------|-----------------|
| ARCH-1 | Systems Architect | Core architecture, security model, platform internals | 39 |
| ARCH-2 | Product Architect | User-facing surfaces, operator experience, integrations | 41 |
| ARCH-3 | Open-Source Architect | Apache/Enterprise boundary, contributor surface, extension points | 38 |
| SWE-1 | Go Backend Engineer | Server internals, provisioner, registry, CLI | 40 |
| SWE-2 | Frontend + Operator UX | Panel SPA, CLI UX, operator workflows | 41 |
| SWE-3 | Data Plane + AI Integration | Embedding, Qdrant, ingestion connectors, RAG pipeline | 36 |

**Total proposals: 235 tickets across 6 agents.**

---

## Round 1: Independent Drafts

Each agent independently produced a ticket set based on the security audit findings, codebase analysis, and open-core boundary definitions.

### ARCH-1 (Systems Architect) — 39 tickets
Core security fixes (SEC-01–SEC-11), operational reliability (OPS-01–OPS-05), data plane hardening (DP-01–DP-05), platform infrastructure (PLAT-01–PLAT-05). Strong emphasis on defense-in-depth and failure mode coverage. Anticipated kills: multi-host event sync, RBAC, K8s plugin, audit trail, overly broad panel tickets, complex webhook pipeline.

### ARCH-2 (Product Architect) — 41 tickets
Full security set plus operator-facing polish: CLI formatting (POLISH-02), startup self-check (POLISH-04), connection status indicator, deployment examples (DOC-04). Widest scope among architects — included 5 boundary clarifications (Open WebUI, K8s, webhook scope, /metrics auth, event log vs audit trail). Anticipated kills: Open WebUI adapter, K8s plugin, complex webhook pipeline, audit trail, multi-instance credential rotation.

### ARCH-3 (Open-Source Architect) — 38 tickets
Most conservative scope. Strong boundary defense — included explicit conformance checks (CONF-01, CONF-02) and contributor-facing documentation (DOC-03, DOC-04). Added REL-03 (release health monitoring) that no other architect proposed. Anticipated kills: K8s plugin, Open WebUI adapter, complex webhook, audit trail, multi-user auth/RBAC.

### SWE-1 (Go Backend) — 40 tickets
Implementation-grounded proposals with specific file:line references. Added PLAT-04 (migration versioning) after identifying the error-swallowing ALTER TABLE pattern in registry.go:72-74. Added PANEL-03 (handleCreate race) after tracing the false-201 code path. Anticipated kills: K8s plugin, Open WebUI, RBAC-adjacent tickets, complex webhook, audit trail.

### SWE-2 (Frontend + Operator UX) — 41 tickets
Largest panel ticket count (PANEL-01–PANEL-09). Proposed SEC-12 (Qdrant collection name escaping) independently — merged into SEC-11 during synthesis. Added OPS-07 (healthz enrichment, merged into DP-03) and OPS-08 (configurable log tail, deferred). Anticipated kills: service worker offline support, framework migration, pushState routing, Qdrant backup automation, WebSocket log streaming, email notifications.

### SWE-3 (Data Plane + AI Integration) — 36 tickets
Deepest data plane coverage: 15 DP-* tickets including DP-06 through DP-15. Identified the chunker strategy gap (DP-11, deferred). Proposed content hash idempotency (DP-10, merged into DP-04) and stale vector cleanup (DP-14, merged into DP-04). Anticipated kills: multi-provider embedding adapters, Qdrant backup automation, streaming ingestion, PDF/DOCX support, filesystem watcher.

---

## Round 2: Synthesis

### Consensus Tickets — 50 kept

Tickets that received majority support (≥4/6 agents proposed substantially the same work) were kept. Tickets with 3/6 support were evaluated on technical merit and dependency impact.

**By domain prefix:**

| Domain | Count | Notes |
|--------|-------|-------|
| SEC | 11 | All security audit findings — unanimous |
| OPS | 6 | Operational reliability — OPS-06 (diagnostics) synthesized from ARCH-2's POLISH-04 |
| DP | 9 | Data plane quality — DP-06 through DP-09 from SWE-3, adopted by panel |
| INT | 2 | Webhook + structured logging |
| PLAT | 4 | Graceful shutdown, migration versioning, tool validation, auto-restart |
| CONF | 2 | Round-trip and security conformance checks |
| REL | 3 | govulncheck, Dependabot, release health monitoring |
| DOC | 4 | Operator handbook, contributor guide, API reference, example deployments |
| PERF | 2 | ValidQueryToken optimization, embedding batching |
| PANEL | 5 | Health history, resource usage, race fix, i18n, create form |
| POLISH | 2 | Rate limiting, CLI output formatting |

### Dropped Tickets — 28 removed

**Merged (11):** Duplicate or overlapping proposals consolidated into a single consensus ticket.
- DP-06 (SWE-3 context cancellation) → merged into DP-01
- DP-10 (content hash) → merged into DP-04
- DP-14 (stale vector cleanup) → merged into DP-04
- OPS-07 (healthz enrichment) → merged into DP-03
- SEC-12 (SWE-2 Qdrant escaping) → merged into SEC-11
- PLAT-03 (SWE-1 backup CLI) → merged into OPS-05
- PLAT-04 (SWE-1 event ordering) → merged into OPS-02
- PLAT-02 (ARCH-3 plugin conformance) → merged into CONF-01
- POLISH-02 (ARCH-2 graceful shutdown drain) → merged into PLAT-02
- POLISH-04 (ARCH-2 startup self-check) → merged into OPS-06
- PLAT-02/03 (SWE-3 skillset contract/binding) → deferred with skillset versioning

**Deferred (17):** Explicitly pushed to v2.x or future backlog with rationale.
- PLAT-05 (request tracing) — OpenTelemetry dependency premature; INT-02 covers most observability value
- DP-07 (recursive directory traversal) — depends on SEC-03; one-directory-per-source sufficient for v1.x
- DP-11 (chunker strategy selection) — requires tokenizer dependency; current 512-rune chunker adequate
- DP-15 (structured embedding error responses) — low priority polish
- OPS-08 (log streaming) — significant SSE complexity; docker logs -f covers the use case
- PANEL-01 (SWE-2: Settings page) — requires config write API, out of scope
- PANEL-02 (SWE-2: connection status indicator) — minor UX improvement
- PANEL-03 (SWE-2: instance deep links) — hash routing polish
- PANEL-04 (SWE-2: auto-refresh toggle) — SSE already provides real-time updates
- PANEL-07 (loading states/error boundaries) — UX polish, not blocking
- PANEL-08 (keyboard navigation/ARIA) — significant effort across 2500-line SPA; v2.x
- PANEL-09 (bulk instance actions) — operator convenience; typical fleet sizes < 20
- PANEL-01 (SWE-1: log streaming via SSE) — same as OPS-08 reasoning
- PANEL-02 (SWE-1: restart API) — destroy + re-provision covers it; PLAT-03 automates the common case
- PLAT-03 (ARCH-3: skillset versioning) — premature complexity; dissent recorded
- PLAT-02 (SWE-3: skillset contract compatibility) — deferred with versioning
- PLAT-03 (SWE-3: data source binding validation) — depends on deferred versioning

---

## Dissents — 6 resolved

### 1. Prometheus /metrics endpoint authentication
- **Split:** ARCH-1, SWE-1, ARCH-2 (unauthenticated) vs SWE-2 (authenticated). ARCH-3, SWE-3 neutral.
- **Resolution:** Unauthenticated. Standard Prometheus convention. Metrics expose operational data (instance counts, latencies) but no secrets. Operators in hostile networks use a reverse proxy with auth.

### 2. admin_secret minimum length: 16 vs 32 characters
- **Split:** ARCH-1, SWE-3 (32 chars) vs SWE-1, ARCH-2, SWE-2 (16 chars). ARCH-3 neutral.
- **Resolution:** 16 characters. 16 hex chars = 64 bits of entropy, sufficient for single-host bearer token brute-force resistance. 32 chars creates operator friction for manual configuration. (Note: the original dissent text incorrectly stated "128 bits if alphanumeric" — actual entropy for 16 alphanumeric chars is ~95 bits. Conclusion unchanged.)

### 3. TLS scope: manual cert only vs ACME auto-cert
- **Split:** ARCH-1 (manual + ACME) vs SWE-1, ARCH-2, SWE-2 (manual only). ARCH-3 (manual only, ACME deferred).
- **Resolution:** Manual cert only for Apache scope. ACME adds autocert dependency, requires port 80 for HTTP-01 challenge, and needs persistent cert storage. Operators needing auto-TLS already have Caddy/Traefik.

### 4. Event retention default: 7 days vs 30 days
- **Split:** ARCH-1, ARCH-2 (30 days) vs SWE-1, SWE-3, ARCH-3 (7 days). SWE-2 (14 days).
- **Resolution:** 7 days default. Persistent event log is for operational visibility, not compliance. Negligible disk usage at typical event rates (<10 MB). Operators can configure up to 365 days. Compliance-grade retention is an Enterprise audit trail feature.

### 5. Skillset version management in Apache backlog
- **Split:** ARCH-3, ARCH-2 (include) vs SWE-1, SWE-3 (defer). ARCH-1 neutral.
- **Resolution:** Deferred. Skillset versioning requires version storage, migration paths, and rollback semantics that are premature for v1.x. Current single-version model works. Dissent recorded — revisit when operators report version conflicts.

### 6. Panel log streaming via SSE
- **Split:** ARCH-2, SWE-1 (include) vs SWE-2 (oppose — duplicates SSE infrastructure). ARCH-3 neutral.
- **Resolution:** Deferred to v2.x. Log streaming adds per-instance SSE channels and Docker log streaming goroutines. Current fetch-based 100-line tail is adequate. Operators needing real-time logs use `docker logs -f` directly.

---

## Boundary Decisions — 3 unanimous

All three boundary decisions were unanimous (6/6 panel agreement). These define the Apache vs Enterprise line for the features most likely to generate scope pressure.

### 1. Open WebUI Adapter → Enterprise
The DeploymentPlugin interface is proven by the Docker plugin and conformance suite. An Open WebUI adapter is a runtime-specific integration requiring ongoing maintenance against a third-party API surface. The plugin development guide (DOC-02) enables third-party contributors to build adapters. The Hermes adapter is the reference implementation; additional adapters are commercial differentiation.

### 2. K8s Deployment Plugin → Enterprise
K8s per-instance pods require pod management, service mesh integration, PVC provisioning, and RBAC — enterprise-scale concerns. The existing Helm chart covers deploying fox-control itself on K8s. The DeploymentPlugin interface is the Apache extensibility point; the K8s implementation is the commercial product. **Boundary: single-host Docker = free, multi-host orchestration = paid.**

### 3. Webhook Forwarding Scope → Split
Simple POST-on-event with HMAC-SHA256 signing is Apache (INT-01). Retry with exponential backoff, dead-letter queue, payload transforms, conditional routing, and multi-endpoint fan-out are Enterprise. The boundary is drawn at delivery semantics: best-effort fire-and-forget is the Apache ceiling. This covers ~80% of use cases (Slack notifications, PagerDuty alerts, simple HTTP integrations) with ~50 lines of Go. SWE-1 estimated Apache webhook at S (1–2 days) vs Enterprise pipeline at L (1–2 weeks).

---

## Adversarial Review

The adversarial reviewer verified source code references and identified 24 findings across 4 categories.

### Blocking Issues — 3 (all resolved in backlog)

1. **SEC-02 — DNS rebinding bypasses SSRF protection.** Original AC validated resolved IP only at Connect time; HTTP requests happen later during Ingest. **Fix applied:** AC now requires custom `net.Dialer` in HTTP client `Transport.DialContext` that validates resolved IP on every TCP connection.

2. **SEC-03 — `strings.HasPrefix` path containment is bypassable.** The path `/allowed/dir-escape` would pass `strings.HasPrefix("/allowed/dir", ...)`. **Fix applied:** AC now requires `allowedDir + string(filepath.Separator)` prefix check, with test case for sibling directory attack.

3. **OPS-05 — `sqlite3_backup_init` unavailable in modernc.org/sqlite.** The CGo-free driver does not expose the C backup API. **Fix applied:** AC now specifies `VACUUM INTO` (available in the SQLite version bundled with modernc).

### Non-Blocking Findings — 21

**Security (4):** PERF-01 constant-time comparison conflict (resolved: hash-then-lookup pattern), SEC-05 blocklist incompleteness noted, OPS-03 /metrics exposure in shared hosting documented, SEC-06 entropy math corrected, INT-01 replay protection gap noted (timestamp header recommended).

**Correctness (8):** SEC-01 tools.json rationale clarified (references env var name, not secret value). OPS-01 recharacterized as documentation bug (port reclamation already works). PANEL-03 reframed (provisioner mutex prevents duplicate containers, but HTTP handler returns false 201). DP-05 dependency on SEC-04 is conceptual not technical. CONF-02 missing OPS-03 dependency noted. DP-04 REST connector incremental logic may be undersized. SEC-04 mandated ResponseController approach over either/or. OPS-02 v1.1.0 velocity noted as aggressive.

**Test discipline (6):** SEC-04 needs WriteTimeout integration test. PLAT-02 needs subprocess-level test specificity. OPS-02 needs async write batching or explicit perf tradeoff. PLAT-03 needs edge case tests (in-flight provision, ErrCapReached, concurrent unhealthy). CONF-01 needs mock infrastructure for CI. DP-04 needs point-level correctness test (not just performance benchmark).

**Code quality (6):** PLAT-04 migration sequence specified. SEC-10 yaml.v3 comment preservation noted. OPS-05 backup mechanism corrected. DOC-01 incremental writing recommended. PANEL-04 i18n approach needs no-build-step compatibility. PERF-02 embedding client timeout should be configurable.

### Verdict
REVISE — 3 blocking issues fixed in the published backlog. Non-blocking findings documented as implementation notes in ticket acceptance criteria where actionable.
