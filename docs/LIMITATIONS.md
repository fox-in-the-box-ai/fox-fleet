# Known Limitations

Fox Fleet v1.4.2 — last updated 2026-06-09.

This document tracks architectural boundaries, known gaps, and constraints
that operators and contributors should be aware of. Items here are
intentional scope limits, not bugs.

---

## Single-host architecture

Fox Fleet manages Docker containers on the host where `fox-control` runs.
There is no multi-host orchestration, container migration, or cross-node
scheduling. For multi-node deployments, run one `fox-control` instance per
host and manage them independently.

**Implication:** horizontal scaling requires external load balancing and
a shared storage layer for the source registry and Qdrant.

## Docker socket dependency

`fox-control` requires direct access to the Docker socket
(`/var/run/docker.sock`). This grants the process full Docker API access,
which is effectively root-equivalent on the host. The systemd unit and
Helm chart apply security hardening (dropped capabilities, no privilege
escalation), but the Docker socket itself is the trust boundary.

**Mitigation:** run `fox-control` under a dedicated system user with only
the `docker` group. Do not expose the management API to untrusted
networks without an authenticating reverse proxy.

## No high availability

`fox-control` runs as a single process with a local SQLite database.
There is no leader election, replication, or automatic failover. If the
process crashes, systemd restarts it; if the host goes down, manual
recovery is required.

**Workaround:** regular backups via `fox-control backup` (VACUUM INTO)
or filesystem snapshot of `/var/lib/fox-control`.

## Qdrant is external

The Helm chart and systemd deployments do not manage Qdrant — operators
must provision and maintain their own Qdrant instance. Docker-based
deployments (Docker Compose and the data-plane `qdrant.Manager`) can
provision and lifecycle-manage a Qdrant container automatically, but it
runs a single un-replicated node with no backup automation.

## Embedding provider

The data plane calls an OpenAI-compatible embedding API. The default
config points at a local Ollama instance. There is no built-in model
download, GPU scheduling, or embedding queue. If the embedding provider
is slow or unavailable, ingestion and queries block or fail. Embedding
calls retry with exponential backoff on 429/5xx errors.

## Source ingestion limits

- **File connector:** 50 MB per file, local filesystem only.
- **REST connector:** 1000 pages maximum, single JSON endpoint per source.
- No streaming ingestion, no webhook-triggered re-ingestion.
- Incremental re-ingestion is supported (SHA-256 content hashing skips
  unchanged files), but schema changes require a full re-ingest.

## Authentication model

All management API endpoints share a single bearer token
(`auth.admin_secret`). There are no user accounts, roles, RBAC, or
multi-user session management. The token is static until manually rotated.

Instance containers receive a shared `instance_password` — there is no
per-instance credential rotation. Per-instance query tokens exist for
data plane access but are not user-facing credentials.

## Skillset validation

Skillset YAML manifests are validated at upload time — `contract_version`
must be valid semver (no specific version restriction).
`ValidateAgainstManifest` checks declared
tools against available tools at provision time. There is no runtime
validation inside the instance that tools are actually functional — a
misconfigured skillset will be accepted but the instance may fail to
use the declared tools.

## Panel SPA

The management panel is a vanilla HTML/CSS/JS single-page application
embedded in the Go binary. It has no offline support, no service worker,
no client-side routing (hash-based navigation only), and no build step.
The panel supports English, Spanish, and French.

---

## v2.x scope (not in Apache Base)

These are tracked as Enterprise or future features, not bugs:

| Limitation | v2.x / Enterprise |
|---|---|
| Single-host only | Multi-node with shared registry |
| No HA | SQLite WAL + read replicas |
| Single auth token | Multi-user with RBAC, SSO/OIDC |
| No ACME auto-cert | Automatic Let's Encrypt via built-in ACME |
| No OpenTelemetry | Request tracing |
| No real-time log streaming | SSE-based log tailing in panel |
| No bulk instance actions | Multi-select provision/destroy |
