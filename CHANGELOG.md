# Changelog

All notable changes to Fox Fleet are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **DP-01:** Qdrant vector DB container management — shared sidecar lifecycle, health polling, auto-start with provisioner (#51)
- **PLAT-01:** Skillset manifest spec — YAML schema, parser, validator, and conformance tests (#51)
- **DP-02:** File ingestion connector — local file/directory upload, chunking, embedding, Qdrant upsert with 50 MB limit
- **DP-03:** REST ingestion connector — paginated JSON API fetch, SSRF protection, 1000-page limit, bearer auth
- **DP-07a:** Source management API — SQLite registry, admin CRUD endpoints with auth, ingest trigger
- Data plane server with health/readyz probes, public source listing, admin auth via `crypto/subtle`
- Text chunker — 512-token fixed-size with 64-token overlap, rune-aware Unicode support
- Embedding client — OpenAI-compatible HTTP API adapter
- Qdrant REST client — collection CRUD, point upsert, vector search with payload filtering
- **DP-05:** Query API — `POST /v1/query` with embedding + vector search, source filtering, top-k control, 503 on infra failure
- **DP-08:** Panel sources view — tabbed UI with Instances/Sources navigation, source table with status badges, auto-refresh
- **PLAT-02:** Hermes adapter — panel wires source registry and data plane URL through to provisioner when data plane is enabled
- **PLAT-03:** Data plane agent plugin — config injection writes `tools.json` with `knowledge_query` tool manifest (URL, auth header, parameters)

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
