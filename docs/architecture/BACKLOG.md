# Fox Fleet — Apache Base Backlog

> 50 tickets from the v1 panel deliberation. Specifications: [APACHE_BACKLOG_v1.md](APACHE_BACKLOG_v1.md). Roadmap: [APACHE_ROADMAP_v1.x.md](APACHE_ROADMAP_v1.x.md).

| ID | Title | Size | Release | Status |
|----|-------|------|---------|--------|
| SEC-01 | Fix hermes.env file permissions (0644 → 0600) | XS | v1.1.0 | Shipped |
| SEC-02 | Harden REST connector against SSRF | S | v1.1.0 | Shipped |
| SEC-03 | Restrict file connector to allowed directory | S | v1.1.0 | Shipped |
| SEC-04 | Add HTTP server WriteTimeout | S | v1.1.0 | Shipped |
| SEC-05 | Validate InstanceConfig.Env against blocklist | XS | v1.1.0 | Shipped |
| SEC-06 | Enforce admin_secret minimum length + generate-secret | XS | v1.1.0 | Shipped |
| SEC-07 | Validate instance ID format | XS | v1.1.0 | Shipped |
| SEC-08 | Add digest-only image reference warning | XS | v1.1.0 | Shipped |
| SEC-09 | Add security headers to embedded SPA | XS | v1.1.0 | Shipped |
| SEC-10 | Replace fmt.Fprintf YAML with yaml.v3 Marshal | XS | v1.1.0 | Shipped |
| SEC-11 | URL-escape Qdrant collection names | XS | v1.1.0 | Shipped |
| OPS-01 | Fix port reclamation on instance destroy | XS | v1.1.0 | Shipped (verified, no change needed) |
| OPS-02 | Persistent event log (SQLite-backed) | M | v1.1.0 | Shipped |
| OPS-03 | Prometheus metrics endpoint | S | v1.2.0 | Shipped |
| OPS-04 | Built-in TLS termination | S | v1.2.0 | Shipped |
| OPS-05 | SQLite backup/restore CLI (VACUUM INTO) | S | v1.2.0 | Shipped |
| OPS-06 | Diagnostics command | M | v1.2.0 | Shipped |
| DP-01 | Embedding client retry with backoff | S | v1.3.0 | Backlog |
| DP-02 | Per-operation Qdrant client timeouts | S | v1.3.0 | Backlog |
| DP-03 | Qdrant health monitoring | S | v1.3.0 | Backlog |
| DP-04 | Incremental source re-ingestion | M | v1.3.0 | Backlog |
| DP-05 | Data plane WriteTimeout and body limits | XS | v1.1.0 | Shipped |
| DP-06 | Qdrant upsert batching | S | v1.3.0 | Backlog |
| DP-07 | Source deletion cascade | S | v1.3.0 | Backlog |
| DP-08 | Embedding dimension validation | S | v1.3.0 | Backlog |
| DP-09 | Query result score threshold filtering | S | v1.3.0 | Backlog |
| INT-01 | Webhook event forwarding (HMAC-SHA256) | M | v1.2.0 | Shipped |
| INT-02 | Structured JSON logging | S | v1.2.0 | Shipped |
| PLAT-01 | Runtime skillset tool validation | S | v1.3.0 | Backlog |
| PLAT-02 | Graceful shutdown with request draining | S | v1.1.0 | Shipped |
| PLAT-03 | Health-check-based auto-restart | M | v1.3.0 | Backlog |
| PLAT-04 | Registry migration versioning | S | v1.1.0 | Shipped |
| CONF-01 | Data plane conformance checks | M | v1.3.0 | Backlog |
| CONF-02 | Security conformance checks | S | v1.3.0 | Backlog |
| REL-01 | govulncheck in CI | S | v1.2.0 | Shipped |
| REL-02 | Automated dependency updates (Dependabot) | S | v1.2.0 | Shipped |
| REL-03 | Release health monitoring | M | v1.3.0 | Backlog |
| DOC-01 | Operator handbook | L | v1.4.0 | Backlog |
| DOC-02 | Developer and contributor handbook | M | v1.4.0 | Backlog |
| DOC-03 | API reference (OpenAPI 3.0) | M | v1.4.0 | Backlog |
| DOC-04 | Example deployment configurations | S | v1.4.0 | Backlog |
| PERF-01 | ValidQueryToken hash-then-lookup optimization | XS | v1.2.0 | Shipped |
| PERF-02 | Embedding request batching | M | v1.3.0 | Backlog |
| PANEL-01 | Instance health history timeline | S | v1.4.0 | Backlog |
| PANEL-02 | Per-instance resource usage display | S | v1.4.0 | Backlog |
| PANEL-03 | Fix handleCreate race condition | S | v1.1.0 | Shipped |
| PANEL-04 | i18n contribution workflow | S | v1.4.0 | Backlog |
| PANEL-05 | Instance create form with skillset dropdown | S | v1.4.0 | Backlog |
| POLISH-01 | Token bucket rate limiting | S | v1.2.0 | Shipped |
| POLISH-02 | CLI --output flag (table/json/quiet) | S | v1.2.0 | Shipped |

**Summary:** 12 XS, 26 S, 10 M, 1 L, 1 unused = 50 tickets across 4 releases (~9 weeks).
