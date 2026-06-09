# Changelog

All notable changes to Fox Fleet are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.4.1] - 2026-06-09

### Fixed

- **C-01:** Wire data-plane HTTP server into `fox-control` binary — was defined but never started
- **C-02:** Wire `AutoRestart` config through `Deps` to `HealthPoller` — threshold and cooldown were ignored
- **C-03:** Run container as non-root user 65532 in Dockerfile
- **C-04:** Set `CGO_ENABLED=0` in Dockerfile for pure-Go modernc/sqlite build
- **C-05:** Remove `MemoryDenyWriteExecute=true` from systemd unit — incompatible with modernc/sqlite JIT
- **C-06:** Replace stale `--dest`/`--from` CLI flag references with `--output`/`--input` in operator handbook
- **C-07:** Correct OpenAPI spec version to 1.4.1 and license to Apache-2.0
- **H-01:** Add Google Fonts domains to CSP `style-src` and `font-src` directives
- **H-02:** Wire `RateLimitSection` from config into `Deps` — rate limit settings were parsed but not forwarded
- **H-03:** Update install docs version references from v1.0.0 to v1.4.1
- **H-04:** Surface Qdrant health in `/healthz` — returns `"degraded"` when Qdrant is unhealthy
- **H-05:** Add `?token=` query-param fallback for SSE auth (EventSource cannot set headers)
- **H-06:** Add actionable CTA to sources empty state with i18n (en/es/fr)
- **H-07:** Add `tabindex` and keyboard event handlers to interactive table rows for accessibility

## [1.4.0] - 2026-06-08

### Added

- **PANEL-01:** Instance health timeline — 24-hour color-coded health history bar in instance detail view; `GET /api/instances/{id}/health-history` endpoint; health transition events emitted by the poller
- **PANEL-02:** Instance resource gauges — CPU, memory, and network usage display with critical/high thresholds; `GET /api/instances/{id}/stats` endpoint; `ContainerStats` struct and `Stats` method on `DeploymentPlugin` interface; Docker implementation via one-shot container stats API
- **PANEL-04:** i18n extraction and French language — translations extracted from inline JS to `panel/spa/static/i18n/{en,es,fr}.json`; locales loaded asynchronously via fetch; `scripts/validate-i18n.sh` for key-parity checks; i18n contribution guide added to `CONTRIBUTING.md`
- **PANEL-05:** Skillset picker dropdown — provision modal now fetches available skillsets from `/api/skillsets` and presents a dropdown instead of free-text input
- **DOC-01:** Operator handbook updated for v1.1.0–v1.4.0 — Prometheus metrics, structured logging, built-in TLS, CLI backup, persistent events, webhooks, rate limiting, auto-restart, health history, resource stats; troubleshooting table expanded to 18 scenarios
- **DOC-02:** Developer handbook — architecture overview, package layout, `DeploymentPlugin` development guide, data plane connector guide, test patterns, build instructions
- **DOC-03:** OpenAPI 3.0.3 specification — 28 operations across panel API and data plane API with shared schemas and security definitions
- **DOC-04:** Example configurations — minimal, production (Docker Compose + TLS + Qdrant), and air-gapped (pre-pulled images + local Ollama + setup script) deployment examples

## [1.3.0] - 2026-06-08

### Added

- **DP-01:** Embedding retry with exponential backoff — retries 429/5xx with jitter, configurable max retries and backoff bounds
- **DP-02:** Per-operation Qdrant timeouts — separate timeouts for health (5s), search (30s), upsert (60s), and collection (30s) operations
- **DP-03:** Qdrant health monitoring — panel health poller checks Qdrant status with state transition logging; `/healthz` surfaces Qdrant health
- **DP-04:** Incremental file ingestion — SHA-256 content hashing skips unchanged documents; removed files cleaned from Qdrant and tracking table
- **DP-06:** Qdrant upsert batching — large point sets split into 100-point batches to avoid request size limits
- **DP-07:** DeleteByFilter for Qdrant — enables cascade deletion of source points during source removal
- **DP-08:** Configurable minimum score threshold — query results below `min_score` (per-request or server default) are filtered out
- **DP-09:** Embedding dimension validation — data plane validates embedding model dimensions against configured vector size at startup
- **PLAT-01:** Skillset tool validation — `ValidateAgainstManifest` reports missing and extra tools against the skillset manifest
- **PLAT-03:** Instance auto-restart — health poller auto-restarts instances exceeding a configurable consecutive failure threshold with cooldown; `DeploymentPlugin.Restart` method added
- **CONF-01:** Data plane conformance suite — 10 round-trip checks covering health, readyz, admin auth, source CRUD, query auth, and content-type compliance
- **CONF-02:** Security conformance checks — 4 checks for security headers, path traversal rejection, auth timing consistency, and HTTP method restriction
- **REL-03:** Release health monitoring workflow — weekly scheduled CI checks tag/CHANGELOG alignment, go.sum freshness, govulncheck, cross-platform build, and Helm version parity
- **PERF-02:** Cross-file embedding batching — ingestion batches up to 256 texts per embed call across files, reducing API round trips

## [1.2.0] - 2026-06-08

### Added

- **PERF-01:** Query token hash index — `ValidQueryToken` now uses SHA-256 hash lookup with index scan instead of full table scan, followed by constant-time comparison
- **POLISH-01:** Token bucket rate limiter — API requests throttled with configurable rate; 429 responses include `Retry-After` header
- **OPS-03:** Prometheus metrics endpoint — `/metrics` exposes request count, error count, provision count, SSE connections, and uptime in Prometheus exposition format
- **OPS-04:** Built-in TLS termination — `tls.cert_file` and `tls.key_file` config options; data plane URL auto-switches to HTTPS when TLS is configured
- **OPS-05:** SQLite backup/restore CLI — `fox-control backup` uses VACUUM INTO for registry.db, sources.db, and events.db; `fox-control restore` with lock-file safety check
- **OPS-06:** Diagnostics command — `fox-control diagnostics` runs 8 health checks (config, Docker, registry integrity, disk space, port availability, qdrant, embedding, data plane)
- **INT-01:** Webhook event forwarding — configurable webhook targets with HMAC-SHA256 signatures (`X-Fox-Signature`), event type filtering, per-target rate limiting (10/sec), 5s timeout
- **INT-02:** Structured JSON logging — `control.log_format` (text/json) and `control.log_level` (debug/info/warn/error) configuration options
- **POLISH-02:** CLI output formatting — `--output` / `-o` flag supporting table, json, and quiet formats
- **REL-01:** govulncheck in CI — new CI job runs `govulncheck ./...` on every push and PR to detect known Go vulnerabilities
- **REL-02:** Docker Dependabot — Dependabot now monitors Docker base image updates alongside Go modules and GitHub Actions

## [1.1.0] - 2026-06-08

### Security

- **SEC-01:** File permissions hardened — secret files (`hermes.env`, `tools.json`) written with 0600, non-secret config (`config.yaml`, `settings.json`) with 0644
- **SEC-02:** SSRF protection — REST ingestion connector uses a custom `net.Dialer` that blocks private, loopback, link-local, and CGNAT addresses
- **SEC-03:** Path traversal guard — file ingestion connector resolves symlinks and enforces `AllowedFileDir` with `filepath.Separator` prefix check
- **SEC-04:** Panel write timeout — `WriteTimeout: 30s` on panel HTTP server; SSE handler extends deadline per-flush via `ResponseController`
- **SEC-05:** Env key blocklist — user-supplied `InstanceConfig.Env` keys validated against a blocklist of reserved names (`FOX_PLANE_AUTH_SECRET`, `PATH`, `LD_PRELOAD`, etc.)
- **SEC-06:** Admin secret minimum length — `admin_secret` must be at least 16 characters; `fox-control generate-secret` command added for secure key generation
- **SEC-07:** Instance ID validation — `handleDetail` and `handleDestroy` now validate instance ID format (was only checked in `handleCreate`)
- **SEC-08:** Digest-only image warning — startup logs a warning when `docker.image` uses a tag without a pinned digest
- **SEC-09:** Security headers — `Content-Security-Policy`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, and `Referrer-Policy` on embedded SPA responses
- **SEC-10:** YAML injection prevention — `config.yaml` rendering switched from `fmt.Fprintf` to `yaml.Marshal` to prevent special-character injection
- **SEC-11:** Qdrant URL encoding — collection names URL-escaped in all Qdrant REST client calls

### Added

- `internal/safedialer` package — reusable SSRF-safe `net.Dialer` with private-IP blocklist
- `fox-control generate-secret` CLI command — generates cryptographically random hex secrets
- `internal/events.Store` — SQLite-backed persistent event storage
- `events.NewPersistentLog` — event log constructor with SQLite persistence; events survive restarts
- Versioned schema migration system for the instance registry (`schema_version` table, per-migration transactions with rollback)

### Changed

- **DP-05:** Data plane write timeout — `WriteTimeout: 300s` added to data plane HTTP server
- **PANEL-03:** Create race fix — in-flight provisioning guard prevents duplicate 201 responses for concurrent requests with the same instance ID
- **PLAT-02:** Graceful shutdown — background provisions tracked with `sync.WaitGroup`; server waits for completion after HTTP shutdown; shutdown timeout increased to 10s
- **PLAT-04:** Registry migrations — replaced ad-hoc `ALTER TABLE` with versioned migration system; each migration runs in its own transaction

### Verified

- **OPS-01:** Port reclamation works correctly — `Destroy` removes registry entry, `UsedPorts` excludes it, `allocatePort` reuses it on next provision. No code change needed.

## [1.0.1] - 2026-06-08

### Security

- **#99:** SSE auth hardened — admin secret no longer transmitted as a URL query parameter; replaced with HMAC-SHA256 signed session tokens delivered via HttpOnly cookie (`fox_sse_token`); configurable TTL via `session_token_ttl_seconds` (default 600s, range 60–3600); signing key stored in registry with `sec rotate-sse-key` rotation command
- **#100:** Data plane query auth — `/v1/query` and `/v1/sources` endpoints now require authentication; each instance receives a unique 32-byte query token at provision time, validated via `Authorization: Bearer` or `X-Fox-Auth` header; admin secret also accepted; `sec rotate-query-token --instance <id>` command for token rotation; existing instances backfilled automatically on upgrade

### Added

- `internal/sessiontoken` package — HMAC-SHA256 token signing with purpose byte, expiry, and nonce
- `POST /api/auth/session` endpoint — issues time-limited SSE session tokens via Set-Cookie
- `sec rotate-sse-key` CLI command — rotates the SSE signing key, invalidating all active session tokens
- `sec rotate-query-token` CLI command — rotates a data plane query token for a specific instance
- `QueryTokenValidator` interface and `WithTokenValidator` option for data plane server constructor

## [1.0.0] - 2026-06-08

### Added

- **REL-02:** Cosign keyless signing — Sigstore cosign signs every release binary, checksums file, and container image via GitHub Actions OIDC; `fox-control verify` subcommand validates signatures locally
- **REL-03:** SBOM generation — CycloneDX JSON SBOMs for binary releases (Go module dependencies) and container images (full image contents), signed with cosign; container SBOM attested via `cosign attest`
- **CONF-03:** Conformance suite in CI — runtime and plugin conformance checks run on every push and PR against a locally-built Docker image
- **PLAT-06:** Contract v2.0 conformance tests — checks 17–20 validate version/capabilities/health/readyz v2.0 response schemas, Content-Type headers, and type constraints
- **PLAT-07:** Skillset admin view (full) — detail view now shows memory provider/config, avatar, UI removals; download YAML button; replace/update button; backend download endpoint
- **TEST-01:** Test hardening — CI runs tests with `-race -shuffle=on -count=1` to detect data races, order-dependent tests, and prevent false cache hits
- **TEST-02:** Test coverage gate — CI collects coverage profile and fails if total statement coverage drops below 45%
- **TEST-03:** End-to-end smoke tests — lifecycle test covering instance create/list/detail/destroy and skillset upload/list/get/download/delete; auth gate test verifying all API endpoints require authentication and /healthz is public
- **TEST-04:** Mutation testing pass — gremlins mutation testing run across critical packages (panel/api, skillsets, internal/registry, internal/events, internal/config, rollout); 89–100% efficacy confirms test suite catches real regressions
- **RELEASE-01:** Release automation hardening — all GitHub Actions pinned to commit SHAs (supply-chain defense); `timeout-minutes` on every job; pre-release verification gate (tag format, CHANGELOG entry, lint, tests) blocks build+release on failure; CHANGELOG extraction fails hard on empty section instead of falling back to generic text
- **RELEASE-02:** Homebrew tap + Debian packages — release workflow builds `.deb` packages for linux/amd64 and linux/arm64 and attaches them to the GitHub release; Homebrew formula template auto-renders and pushes to `homebrew-fox-fleet` tap repo on stable releases; `install.sh` convenience script detects OS/arch and downloads the correct binary from GitHub Releases
- **RELEASE-03:** Documentation site — mkdocs-material site with installation guide, configuration reference, walkthrough, deployment guide, security docs, and changelog; GitHub Pages deployment workflow triggered on docs changes; Mermaid diagram support

### Changed

- **POLISH-GA-01:** Error message audit — all user-facing error messages (API responses and CLI output) rewritten for consistency, actionability, and relevant context; config validation errors now suggest how to fix each issue
- **POLISH-GA-02:** Performance benchmarks — Go benchmarks for registry operations, event log, skillset parsing, and API serialization; `make bench` target; replaced O(n) duplicate scan in instance creation with primary-key lookup
- **POLISH-GA-03:** Configuration validation hardening — port range bounds [1, 65535], max_instances capped at 1000, listen address parsing via `net.SplitHostPort`, port collision detection across control/instances/qdrant/data_plane, qdrant.image required when enabled, health_poll_seconds bounds [1, 3600]
- **POLISH-GA-04:** License compliance + attribution — NOTICE file (Apache 2.0 Section 4(d)), THIRD-PARTY-LICENSES with full dependency inventory and license texts for all 40 dependencies (6 direct, 34 indirect); all dependencies verified Apache-2.0/MIT/BSD compatible

## [0.3.0-alpha] - 2026-06-08

### Added

- **UI-01:** Branded panel redesign — Fox palette, Sora/Manrope typography, card-based dashboard (#57)
- **UI-02:** Skillset management UI — create, view, delete skillsets from the panel (#58)
- **UI-03:** Knowledge query playground — interactive query interface with data plane proxy (#59)
- **UI-04:** Source detail view — clickable source rows, full field display with status badges (#60)
- **UI-05:** Activity feed — event log with ring buffer, newest-first event table (#61)
- **DEPLOY-01:** Container image — multi-stage Alpine Dockerfile, multi-arch (amd64/arm64), GHCR release workflow (#62)
- **DEPLOY-02:** Docker Compose stack — fox-control + Qdrant, health checks, env-based secrets (#63)
- **DEPLOY-03:** Helm chart — full Kubernetes deployment with ConfigMap, Secret (existingSecret support), PVC, Ingress, security context, `/healthz` probe endpoint (#64)
- `/healthz` unauthenticated health endpoint for Kubernetes probes
- Environment variable overrides for `FOX_ADMIN_SECRET` and `FOX_INSTANCE_PASSWORD` (takes precedence over TOML config)
- **DEPLOY-04:** systemd unit and install script — dedicated system user, security hardening, env-based secrets, idempotent installer (#65)
- **EDGE-BASE-01:** Caddy reverse proxy — automatic TLS, panel + data plane routing, instance subdomain pattern (#66)
- **EDGE-BASE-02:** Documented limitations — architectural boundaries, known gaps, planned improvements (#67)
- **POLISH-01:** Internationalization — `t()` i18n function, English + Spanish dictionaries, `data-i18n` attribute binding, language selector with localStorage persistence, locale-aware date formatting
- **POLISH-02:** Dark mode — CSS custom property theming, `[data-theme="dark"]` override layer, system preference detection via `prefers-color-scheme`, theme selector with localStorage persistence
- **POLISH-03:** Mobile-responsive layout — hamburger sidebar toggle, `@media (max-width: 768px)` breakpoint, off-canvas sidebar with overlay, single-column card grid, responsive detail views/tables/modals/toasts
- **POLISH-04:** Real-time updates via SSE — `GET /api/events/stream` Server-Sent Events endpoint with ring-buffer replay via `Last-Event-ID`, pub/sub fan-out, query-param auth for EventSource, debounced UI refresh, automatic polling fallback on connection loss
- **DOC-01:** End-to-end deployment guide — Docker Compose, Helm, systemd, and manual binary methods with configuration reference, verification steps, upgrade procedures, and troubleshooting
- **DOC-02:** Walkthrough — 12-scene step-by-step guide from first launch through provisioning, knowledge ingestion, theming, mobile layout, real-time updates, and teardown

## [0.2.0-alpha] - 2026-06-08

### Added

- **DP-01:** Qdrant vector DB container management — shared sidecar lifecycle, health polling, auto-start with provisioner (#51)
- **PLAT-01:** Skillset manifest spec — YAML schema, parser, validator, and conformance tests (#51)
- **DP-02:** File ingestion connector — local file/directory upload, chunking, embedding, Qdrant upsert with 50 MB limit (#52)
- **DP-03:** REST ingestion connector — paginated JSON API fetch, SSRF protection, 1000-page limit, bearer auth (#52)
- **DP-07a:** Source management API — SQLite registry, admin CRUD endpoints with auth, ingest trigger (#52)
- Data plane server with health/readyz probes, public source listing, admin auth via `crypto/subtle` (#52)
- Text chunker — 512-token fixed-size with 64-token overlap, rune-aware Unicode support (#52)
- Embedding client — OpenAI-compatible HTTP API adapter (#52)
- Qdrant REST client — collection CRUD, point upsert, vector search with payload filtering (#52)
- **DP-05:** Query API — `POST /v1/query` with embedding + vector search, source filtering, top-k control, 503 on infra failure (#53)
- **DP-08:** Panel sources view — tabbed UI with Instances/Sources navigation, source table with status badges, auto-refresh (#54)
- **PLAT-02:** Hermes adapter — panel wires source registry and data plane URL through to provisioner when data plane is enabled (#54)
- **PLAT-03:** Data plane agent plugin — config injection writes `tools.json` with `knowledge_query` tool manifest (URL, auth header, parameters) (#54)
- **PLAT-10:** Skillset + role assignment — provisioner validates and copies skillset manifests, registry stores skillset name and principal role, panel create accepts optional skillset path and role with config defaults, detail view shows assigned skillset and role (#55)

## [0.1.0-alpha] - 2026-06-08

### Added

- **PLUG-01:** `DeploymentPlugin` 7-operation Go interface and shared types (#32)
- **CTRL-02:** Per-instance config injection — writes `config.yaml`, `settings.json`, `hermes.env` per data dir (#34)
- **CTRL-01:** SQLite instance registry with WAL mode, CGO-free via `modernc.org/sqlite` (#33)
- **PLUG-02:** Docker plugin implementation — all 7 `DeploymentPlugin` operations (#35)
- **CTRL-03:** Provisioning loop orchestrator with mutex, port allocation, and rollback (#37)
- **CTRL-04:** CLI entry point (`fox-control`) with TOML config parsing and cobra subcommands (#38)
- **CONF-01:** Runtime conformance test suite — 16 checks covering boot invariant, auth, health, readyz, version, capabilities, SSE, and contract version (#40)
- **PANEL-01:** Dashboard REST API — 4 endpoints, health poller, Bearer auth with `crypto/subtle.ConstantTimeCompare` (#41)
- **PANEL-02:** Embedded SPA dashboard — instance grid, detail view, create/destroy, auto-refresh, XSS defense-in-depth (#42)
- **REL-01:** Fleet rollout orchestrator — sequential rolling update with health-gating, automatic rollback, bounded timeouts (#43)
- **CONF-02:** Plugin conformance test suite — 8 checks covering lifecycle, idempotency, and error handling (#44)

### Not yet included

- Container image publishing (DEPLOY-01, shipped in v0.3.0-alpha)
- cosign signature verification (REL-02, shipped in v1.0.0)
- SBOM generation (REL-03, shipped in v1.0.0)
