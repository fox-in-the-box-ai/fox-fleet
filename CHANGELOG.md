# Changelog

All notable changes to Fox Fleet are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
