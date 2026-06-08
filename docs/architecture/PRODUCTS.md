# Fox Fleet — Product Boundary (Apache Base vs Enterprise)

> Authoritative open-core boundary table. ADR: [0014-apache-enterprise-boundary-v1.1.md](adr/0014-apache-enterprise-boundary-v1.1.md).

## Deployment & Orchestration

| Capability | Apache Base | Enterprise |
|-----------|-------------|------------|
| Single-host Docker management | Yes | Yes |
| Helm chart for fox-control on K8s | Yes | Yes |
| K8s per-instance pod deployment plugin | — | Yes |
| Multi-host orchestration | — | Yes |
| Air-gapped deployment examples | Yes (DOC-04) | Yes |

## Authentication & Authorization

| Capability | Apache Base | Enterprise |
|-----------|-------------|------------|
| Single bearer-token admin auth | Yes | Yes |
| Per-instance query tokens | Yes | Yes |
| HMAC-SHA256 session tokens (SSE) | Yes | Yes |
| admin_secret minimum length enforcement | Yes (SEC-06) | Yes |
| Multi-user authentication | — | Yes |
| RBAC (role-based access control) | — | Yes |
| SSO / SAML / OIDC | — | Yes |

## Security

| Capability | Apache Base | Enterprise |
|-----------|-------------|------------|
| SSRF protection (REST connector) | Yes (SEC-02) | Yes |
| Path traversal protection (file connector) | Yes (SEC-03) | Yes |
| HTTP WriteTimeout | Yes (SEC-04) | Yes |
| Security headers (CSP, X-Frame-Options, etc.) | Yes (SEC-09) | Yes |
| Env key blocklist validation | Yes (SEC-05) | Yes |
| Digest-only image warning | Yes (SEC-08) | Yes |
| govulncheck in CI | Yes (REL-01) | Yes |
| Tamper-evident audit trail | — | Yes |
| Compliance logging (SOC 2, HIPAA) | — | Yes |

## Observability & Operations

| Capability | Apache Base | Enterprise |
|-----------|-------------|------------|
| Persistent event log (7-day default) | Yes (OPS-02) | Yes |
| Prometheus metrics (/metrics) | Yes (OPS-03) | Yes |
| Structured JSON logging | Yes (INT-02) | Yes |
| Diagnostics command | Yes (OPS-06) | Yes |
| SQLite backup/restore (VACUUM INTO) | Yes (OPS-05) | Yes |
| Built-in TLS termination (manual cert) | Yes (OPS-04) | Yes |
| ACME auto-cert (Let's Encrypt) | — | Yes |
| Request tracing (OpenTelemetry) | — | Yes |
| Cross-host event synchronization | — | Yes |
| Centralized log aggregation | — | Yes |

## Webhooks & Integrations

| Capability | Apache Base | Enterprise |
|-----------|-------------|------------|
| POST-on-event with HMAC-SHA256 signing | Yes (INT-01) | Yes |
| Retry with exponential backoff | — | Yes |
| Dead-letter queue | — | Yes |
| Payload transforms | — | Yes |
| Conditional routing | — | Yes |
| Multi-endpoint fan-out | — | Yes |

## Runtime Adapters

| Capability | Apache Base | Enterprise |
|-----------|-------------|------------|
| Hermes adapter (reference implementation) | Yes | Yes |
| DeploymentPlugin interface + dev guide | Yes (DOC-02) | Yes |
| Open WebUI adapter | — | Yes |
| LM Studio adapter | — | Yes |
| Custom runtime adapters (third-party) | Via plugin interface | Via plugin interface |

## Data Plane

| Capability | Apache Base | Enterprise |
|-----------|-------------|------------|
| File + REST ingestion connectors | Yes | Yes |
| Embedding retry with backoff | Yes (DP-01) | Yes |
| Incremental re-ingestion (content hashing) | Yes (DP-04) | Yes |
| Qdrant health monitoring | Yes (DP-03) | Yes |
| Score threshold filtering | Yes (DP-09) | Yes |
| Embedding dimension validation | Yes (DP-08) | Yes |
| Multi-provider embedding adapters | — | Yes |
| Advanced chunking strategies | — | Yes |
| PDF/DOCX/HTML format support | — | Yes |

## Panel (Embedded SPA)

| Capability | Apache Base | Enterprise |
|-----------|-------------|------------|
| Instance list, detail, provision, destroy | Yes | Yes |
| Health history timeline | Yes (PANEL-01) | Yes |
| Resource usage display | Yes (PANEL-02) | Yes |
| Skillset dropdown in create form | Yes (PANEL-05) | Yes |
| i18n contribution workflow | Yes (PANEL-04) | Yes |
| Settings page (config write API) | — | Yes |
| Real-time log streaming (SSE) | — | Yes |
| Bulk instance actions | — | Yes |

## Platform & Quality

| Capability | Apache Base | Enterprise |
|-----------|-------------|------------|
| Graceful shutdown | Yes (PLAT-02) | Yes |
| Migration versioning | Yes (PLAT-04) | Yes |
| Auto-restart (health-based, default off) | Yes (PLAT-03) | Yes |
| Skillset tool validation | Yes (PLAT-01) | Yes |
| Rate limiting (API) | Yes (POLISH-01) | Yes |
| Conformance test suite | Yes (CONF-01/02) | Yes |
| Skillset version management | — | Yes |

## Documentation

| Capability | Apache Base | Enterprise |
|-----------|-------------|------------|
| Operator handbook | Yes (DOC-01) | Yes |
| Developer/contributor handbook | Yes (DOC-02) | Yes |
| API reference (OpenAPI 3.0) | Yes (DOC-03) | Yes |
| Example deployments | Yes (DOC-04) | Yes |

---

**Boundary principle:** The Apache Base ships a complete, secure, single-host management plane. Enterprise adds multi-host orchestration, advanced integrations, compliance features, and managed runtime adapters. The `DeploymentPlugin` interface ensures third-party extensibility without requiring Enterprise for custom adapters.
