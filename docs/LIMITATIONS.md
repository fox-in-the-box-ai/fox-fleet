# Known Limitations

Fox Fleet v0.3.0-alpha — last updated 2026-06-08.

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

**Workaround:** regular backups of `/var/lib/fox-control` (the SQLite
database and instance state).

## Instance port allocation

Ports are allocated sequentially from `instances.port_start` (default
8787). There is no port reclamation — if instance A was on port 8787 and
is destroyed, that port is not reused until `fox-control` restarts and
reassigns from the current registry state.

## Qdrant is external

The Helm chart and systemd deployment do not manage Qdrant. Operators
must provision and maintain their own Qdrant instance. The Docker Compose
stack includes Qdrant as a convenience, but it runs a single un-replicated
node with no backup automation.

## Embedding provider

The data plane calls an OpenAI-compatible embedding API. The default
config points at a local Ollama instance. There is no built-in model
download, GPU scheduling, or embedding queue. If the embedding provider
is slow or unavailable, ingestion and queries block or fail.

## Source ingestion limits

- **File connector:** 50 MB per file, local filesystem only.
- **REST connector:** 1000 pages maximum, single JSON endpoint per source.
- No streaming ingestion, no webhook-triggered re-ingestion, no
  incremental updates (full re-ingest on change).

## Authentication model

All management API endpoints share a single bearer token
(`auth.admin_secret`). There are no user accounts, roles, RBAC, or
session management. The token is static until manually rotated.

Instance containers receive a shared `instance_password` — there is no
per-instance credential rotation.

## No TLS termination

`fox-control` serves plain HTTP. TLS must be terminated by a reverse
proxy (Caddy, nginx, Traefik) or cloud load balancer. The Caddy
reference config in `deploy/caddy/` handles this with automatic Let's
Encrypt certificates.

## Event log

The activity feed uses an in-memory ring buffer (200 events). Events are
lost on restart. There is no persistent event store, no webhook
forwarding, and no structured log export.

## Panel SPA

The management panel is a vanilla HTML/CSS/JS single-page application
embedded in the Go binary. It has no offline support, no service worker,
no client-side routing (hash-based navigation only), and no build step.
The panel is English-only with no i18n support (planned for v0.3.0).

## Skillset validation

Skillset YAML manifests are validated at upload time against the
`contract_version: "1"` schema. There is no runtime validation that the
tools declared in a skillset are actually available in the instance image.
A misconfigured skillset will be accepted but the instance may fail to
use the declared tools.

---

## Planned improvements

These limitations are tracked in the project roadmap. Items marked with a
ticket reference are scheduled; others are aspirational.

| Limitation | Planned | Ticket |
|---|---|---|
| Single-host only | Multi-node with shared registry | — |
| No HA | SQLite WAL + read replicas | — |
| Single auth token | Multi-user with RBAC | — |
| Event log volatile | Persistent event store | — |
| English-only panel | i18n (POLISH-01) | Planned |
| No dark mode | Dark mode toggle (POLISH-02) | Planned |
| Desktop-only panel | Mobile responsive (POLISH-03) | Planned |
| Polling refresh | SSE real-time updates (POLISH-04) | Planned |
