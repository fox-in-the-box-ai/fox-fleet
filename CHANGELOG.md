# Changelog

All notable changes to Fox Fleet are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **REL-02:** Cosign keyless signing — Sigstore cosign signs every release binary, checksums file, and container image via GitHub Actions OIDC; `fox-control verify` subcommand validates signatures locally
- **REL-03:** SBOM generation — CycloneDX JSON SBOMs for binary releases (Go module dependencies) and container images (full image contents), signed with cosign; container SBOM attested via `cosign attest`
- **CONF-03:** Conformance suite in CI — runtime and plugin conformance checks run on every push and PR against a locally-built Docker image
- **PLAT-06:** Contract v2.0 conformance tests — checks 17–20 validate version/capabilities/health/readyz v2.0 response schemas, Content-Type headers, and type constraints
- **PLAT-07:** Skillset admin view (full) — detail view now shows memory provider/config, avatar, UI removals; download YAML button; replace/update button; backend download endpoint

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

- Container image publishing (deferred to v0.1.0 stable)
- cosign signature verification (REL-02, deferred to v1.0)
- SBOM generation (REL-03, deferred to v1.0)
