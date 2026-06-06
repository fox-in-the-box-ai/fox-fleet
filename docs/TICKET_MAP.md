# Ticket → Package Map

Engineer's entry point: "I'm implementing ticket X, where does the code go?"

## v0.1 Tickets (17)

| Ticket | Package(s) | Notes |
|--------|-----------|-------|
| FLEET-PRE-01 | n/a (separate `fox-contracts` repo) | Precondition: extract contract schemas before CONF-01 starts |
| INST-01 | (fox-in-the-box repo: `packages/fox-overlay/`) | Fox prereq: `/readyz` endpoint |
| INST-02 | (fox-in-the-box repo: `packages/fox-overlay/`) | Fox prereq: `/version` endpoint |
| INST-03 | (fox-in-the-box repo: `packages/fox-overlay/`) | Fox prereq: `/capabilities` + auth gating |
| AUTH-01 | (fox-in-the-box repo: `packages/fox-overlay/`) | Fox prereq: `check_auth` substitution |
| AUTH-02 | (fox-in-the-box repo: `packages/fox-overlay/`) | Fox prereq: boot-time invariant |
| PLUG-01 | `plugins/` | DeploymentPlugin interface + types |
| PLUG-02 | `plugins/docker/` | Docker plugin implementation |
| CTRL-01 | `internal/registry/` | SQLite instance registry |
| CTRL-02 | `internal/config/` | Config injection (hermes.env, config.yaml, settings.json) |
| CTRL-03 | `internal/provisioner/`, `internal/registry/` | Provisioning loop: port alloc → config → plugin → health → registry |
| CTRL-04 | `cmd/fox-control/`, `internal/config/` | CLI entry point + TOML parsing |
| PANEL-01 | `panel/api/` | Dashboard REST API |
| PANEL-02 | `panel/spa/` | Dashboard SPA (embedded HTML/JS) |
| CONF-01 | `conformance/runtime/` | 16 runtime conformance checks |
| CONF-02 | `conformance/plugin/` | 8 plugin conformance checks |
| REL-01 | `rollout/` | Fleet rollout orchestration (CLI) |

## v0.2 Tickets (10)

| Ticket | Package(s) | Notes |
|--------|-----------|-------|
| DP-01 | `data-plane/` | Shared Qdrant container management |
| DP-02 | `data-plane/` | File upload ingestion connector |
| DP-03 | `data-plane/` | REST API ingestion connector |
| DP-05 | `data-plane/` | Query API (Fleet mode: unfiltered) |
| DP-07a | `data-plane/` | Source management API |
| DP-08 | `panel/spa/`, `panel/api/` | Data sources admin view |
| PLAT-01 | `skillsets/` | Skillset manifest spec + validation |
| PLAT-02 | `skillsets/` | Hermes adapter (config translator) |
| PLAT-03 | (fox-in-the-box repo: `packages/fox-overlay/`) | Data plane agent plugin |
| PLAT-10 | `internal/provisioner/`, `skillsets/` | Skillset + role assignment in provisioning |

## v1.0 Tickets (5)

| Ticket | Package(s) | Notes |
|--------|-----------|-------|
| CONF-03 | `.github/workflows/` | CI workflow for conformance |
| REL-02 | `rollout/` | cosign signature verification |
| REL-03 | `.github/workflows/` | SBOM generation + attachment |
| PLAT-06 | `conformance/runtime/` | Contract v2.0 conformance tests |
| PLAT-07 | `panel/spa/`, `panel/api/` | Skillset admin view |
