# Apache Fleet Base — Forward Backlog v1.x

> Post-v1.0.1 consensus backlog produced by a 6-agent panel deliberation.
> 50 tickets across 12 domain prefixes. 28 proposals dropped with documented reasons.
> All tickets are implementation-ready with acceptance criteria, package targets, and dependency chains.

## Ticket Index

| ID | Title | Package | Size | Deps | Release |
|----|-------|---------|------|------|---------|
| SEC-01 | Fix hermes.env file permissions (0644 → 0600) | internal/config | XS | — | v1.1.0 |
| SEC-02 | Add SSRF protection to REST ingestion connector | data-plane/ingestion/rest | S | — | v1.1.0 |
| SEC-03 | Restrict file connector to configurable allowed directory | data-plane/ingestion/file | S | — | v1.1.0 |
| SEC-04 | Add WriteTimeout to HTTP server | cmd/fox-control | XS | — | v1.1.0 |
| SEC-05 | Validate InstanceConfig.Env keys against blocklist | plugins/docker | S | — | v1.1.0 |
| SEC-06 | Enforce minimum length for admin_secret | cmd/fox-control | XS | — | v1.1.0 |
| SEC-07 | Validate instance ID in handleDetail/handleDestroy | panel/api | XS | — | v1.1.0 |
| SEC-08 | Enforce digest-only image references outside rollout | cmd/fox-control | S | — | v1.1.0 |
| SEC-09 | Add security headers to embedded SPA | panel/api | XS | — | v1.1.0 |
| SEC-10 | Use yaml.v3 encoder for config.yaml rendering | internal/config | S | — | v1.1.0 |
| SEC-11 | URL-escape Qdrant collection name in API paths | data-plane/qdrant | XS | — | v1.1.0 |
| OPS-01 | Implement port reclamation on instance destroy | internal/provisioner | S | — | v1.1.0 |
| OPS-02 | Persist event log to SQLite with configurable retention | internal/events | M | — | v1.1.0 |
| OPS-03 | Add Prometheus metrics endpoint (/metrics) | panel/api | M | — | v1.2.0 |
| OPS-04 | Add built-in TLS termination | cmd/fox-control | M | — | v1.2.0 |
| OPS-05 | Implement SQLite backup/restore CLI commands | cmd/fox-control | S | — | v1.2.0 |
| OPS-06 | Add fox-control diagnostics command | cmd/fox-control | S | — | v1.2.0 |
| DP-01 | Add retry with exponential backoff to embedding client | data-plane/embedding | S | — | v1.3.0 |
| DP-02 | Add configurable timeout to Qdrant client operations | data-plane/qdrant | S | — | v1.3.0 |
| DP-03 | Add Qdrant health monitoring to health poller | panel/api | S | DP-02 | v1.3.0 |
| DP-04 | Implement incremental source re-ingestion with content hashing | data-plane/ingestion | M | DP-01 | v1.3.0 |
| DP-05 | Add data plane WriteTimeout and request body limits | data-plane/server | XS | — | v1.1.0 |
| DP-06 | Add Qdrant upsert batching for large ingestions | data-plane/qdrant | S | — | v1.3.0 |
| DP-07 | Add source deletion cascade to remove Qdrant points | data-plane/server | S | — | v1.3.0 |
| DP-08 | Add embedding dimension validation on startup | data-plane/server | S | — | v1.3.0 |
| DP-09 | Add query result score threshold filtering | data-plane/server | XS | — | v1.3.0 |
| INT-01 | Implement webhook forwarding (POST-on-event) | internal/events | M | OPS-02 | v1.2.0 |
| INT-02 | Add structured JSON logging option | cmd/fox-control | S | — | v1.2.0 |
| PLAT-01 | Add runtime validation of skillset-declared tools | skillsets | M | — | v1.3.0 |
| PLAT-02 | Implement graceful shutdown with in-flight request draining | cmd/fox-control | S | — | v1.1.0 |
| PLAT-03 | Add health-check-based auto-restart for unhealthy instances | panel/api | M | — | v1.3.0 |
| PLAT-04 | Add registry database migration versioning | internal/registry | S | — | v1.1.0 |
| CONF-01 | Add data plane round-trip conformance checks | conformance | M | — | v1.3.0 |
| CONF-02 | Add security-focused conformance checks | conformance/runtime | S | SEC-09 | v1.3.0 |
| REL-01 | Add govulncheck to CI pipeline | build/ci | S | — | v1.2.0 |
| REL-02 | Add automated dependency update workflow | build/ci | S | — | v1.2.0 |
| REL-03 | Add release health monitoring workflow | build/ci | S | REL-01 | v1.3.0 |
| DOC-01 | Write operator handbook | docs | L | OPS-02, OPS-03, OPS-05 | v1.4.0 |
| DOC-02 | Write developer and contributor handbook | docs | M | — | v1.4.0 |
| DOC-03 | Add API reference documentation | docs | M | — | v1.4.0 |
| DOC-04 | Add example deployment configurations | docs | S | — | v1.4.0 |
| PERF-01 | Optimize ValidQueryToken with indexed lookup | internal/registry | XS | — | v1.2.0 |
| PERF-02 | Batch embedding requests in ingestion connectors | data-plane/ingestion | M | DP-01 | v1.3.0 |
| PANEL-01 | Add instance health history and uptime indicator | panel/spa | S | OPS-02 | v1.4.0 |
| PANEL-02 | Add instance resource usage display | panel/api | M | — | v1.4.0 |
| PANEL-03 | Prevent race condition in handleCreate duplicate check | panel/api | XS | — | v1.1.0 |
| PANEL-04 | Add panel i18n contribution workflow | panel/spa | S | — | v1.4.0 |
| PANEL-05 | Add instance create form with skillset/role selection | panel/spa | S | — | v1.4.0 |
| POLISH-01 | Add rate limiting to panel API endpoints | panel/api | S | — | v1.2.0 |
| POLISH-02 | Add CLI output formatting: table, JSON, quiet modes | cmd/fox-control | S | — | v1.2.0 |

---

## Ticket Specifications

### SEC-01: Fix hermes.env file permissions (0644 → 0600)

**Package:** `internal/config` | **Size:** XS | **Release:** v1.1.0 | **Audit:** H1

internal/config/inject.go `writeIfChanged` at line 183 uses `os.WriteFile` with 0644 permissions for all config files, including hermes.env which contains `FOX_PLANE_AUTH_SECRET` and `HERMES_WEBUI_PASSWORD`. Use 0600 for files containing secrets (hermes.env, tools.json), keep 0644 for non-secret files (config.yaml, settings.json).

**Acceptance criteria:**
- `writeIfChanged` uses 0600 for hermes.env (contains `FOX_PLANE_AUTH_SECRET`, `HERMES_WEBUI_PASSWORD`)
- tools.json written with 0600 (contains `FOX_DATA_PLANE_TOKEN` reference)
- config.yaml and settings.json remain 0644 (no secrets)
- New test asserts file mode on each rendered file
- Existing tests pass unchanged

**Panel consensus:** Unanimous 6/6.

---

### SEC-02: Add SSRF protection to REST ingestion connector

**Package:** `data-plane/ingestion/rest` | **Size:** S | **Release:** v1.1.0 | **Audit:** H3

data-plane/ingestion/rest/connector.go lines 43–49 accepts arbitrary URLs from source config. Add `validateOriginURL` that parses the URL, rejects non-http/https schemes, and blocks RFC-1918/link-local/loopback addresses. **Critical: use a custom `net.Dialer` in the HTTP client's `Transport.DialContext` to validate the resolved IP on every TCP connection** — validation at `Connect` time only is insufficient because DNS can rebind between Connect and Ingest.

**Acceptance criteria:**
- `validateOriginURL` rejects non-http/https schemes
- URL host resolved via `net.LookupHost`; RFC-1918, link-local (169.254.x.x), and loopback addresses rejected
- Cloud metadata endpoints (169.254.169.254) explicitly blocked
- HTTP client uses a custom Dialer that rejects private/loopback on every DNS resolution, not just at Connect time
- Unit tests cover: valid URL, localhost, 10.0.0.1, 192.168.1.1, 169.254.169.254, ftp://example.com, file:///etc/passwd
- Existing REST connector tests pass

**Panel consensus:** Unanimous 6/6.

---

### SEC-03: Restrict file connector to configurable allowed directory

**Package:** `data-plane/ingestion/file` | **Size:** S | **Release:** v1.1.0 | **Audit:** H4

data-plane/ingestion/file/connector.go lines 36–49 passes user-supplied config.path to `os.ReadFile`. Add `allowedDir` field from config, resolve both paths with `filepath.Abs` and `filepath.EvalSymlinks`, verify the requested path is under `allowedDir`. **Critical: containment check must use `allowedDir + string(filepath.Separator)` as prefix** — bare `strings.HasPrefix` is bypassable with sibling directories (e.g., `/data/files-secret` would pass a check against `/data/files`).

**Acceptance criteria:**
- File connector has an `allowedDir` field set from config (`data_plane.file_root`, default `data_root/files`)
- `filepath.Abs` and `filepath.EvalSymlinks` applied to both `allowedDir` and requested path
- Cleaned path must equal `allowedDir` OR have `allowedDir + string(filepath.Separator)` as prefix
- Symlink traversal outside `allowedDir` is rejected
- Unit tests cover: valid path, `../../../etc/passwd`, symlink escape, path with spaces, sibling directory with matching prefix
- Existing file connector tests pass

**Panel consensus:** Unanimous 6/6.

---

### SEC-04: Add WriteTimeout to HTTP server

**Package:** `cmd/fox-control` | **Size:** XS | **Release:** v1.1.0 | **Audit:** M1

cmd/fox-control/main.go sets `ReadHeaderTimeout` but no `WriteTimeout`. Use `http.ResponseController.SetWriteDeadline` per-request in the SSE handler to extend the deadline, while setting a global `WriteTimeout` for all other endpoints.

**Acceptance criteria:**
- http.Server in main.go has `WriteTimeout` set (30–60s)
- SSE handler uses `http.ResponseController.SetWriteDeadline` to extend per-flush
- Non-SSE endpoints have a reasonable write deadline
- Integration test: slow non-SSE client disconnected after timeout; SSE client survives beyond global timeout
- Existing tests pass

**Panel consensus:** Unanimous 6/6.

---

### SEC-05: Validate InstanceConfig.Env keys against blocklist

**Package:** `plugins/docker` | **Size:** S | **Release:** v1.1.0 | **Audit:** M4

plugins/docker/plugin.go lines 58–60 iterates `InstanceConfig.Env` without validation. Add a blocklist check before container creation.

**Acceptance criteria:**
- Blocklist: `PATH`, `LD_PRELOAD`, `LD_LIBRARY_PATH`, `HOME`, `USER`, `SHELL`, and any key starting with `FOX_` or `HERMES_`
- Validation runs in Docker plugin before container creation
- Rejected keys produce a clear error message naming the blocked key
- Blocklist is a package-level var for testability
- Unit tests: valid env, blocked `PATH`, blocked `FOX_PLANE_AUTH_SECRET`, blocked `LD_PRELOAD`

**Panel consensus:** 5/6 agreement.

---

### SEC-06: Enforce minimum length for admin_secret

**Package:** `cmd/fox-control` | **Size:** XS | **Release:** v1.1.0 | **Audit:** M5

cmd/fox-control/config.go validates `admin_secret` only for non-empty. Enforce minimum 16 characters. Add `fox-control generate-secret` subcommand.

**Acceptance criteria:**
- `validateConfig` rejects `admin_secret` shorter than 16 characters
- Error message is actionable: specifies minimum length and suggests `openssl rand -hex 32`
- `fox-control generate-secret` outputs a `crypto/rand` 32-byte hex string
- Existing tests updated for minimum length validation

**Panel consensus:** Unanimous 6/6. Dissent on 16 vs 32 resolved: 16 characters provides sufficient entropy for bearer token brute-force resistance on a single-host management plane.

---

### SEC-07: Validate instance ID in handleDetail and handleDestroy

**Package:** `panel/api` | **Size:** XS | **Release:** v1.1.0 | **Audit:** I1

panel/api/handlers.go `handleDetail` and `handleDestroy` do not validate the instance ID against the `validInstanceID` regex (already defined and used in `handleCreate`). Add validation at the top of both handlers.

**Acceptance criteria:**
- `handleDetail` and `handleDestroy` validate instance ID against `validInstanceID` regex
- Invalid IDs return 400 Bad Request with structured error
- `validInstanceID` regex reused from `handleCreate` (no duplication)
- Unit tests: valid ID, empty string, path traversal attempt, SQL injection attempt

**Panel consensus:** Unanimous 6/6.

---

### SEC-08: Enforce digest-only image references outside rollout

**Package:** `cmd/fox-control` | **Size:** S | **Release:** v1.1.0 | **Audit:** I2

The rollout command enforces digest references but config validation does not. Add a warning at startup when the configured image uses a mutable tag. Non-breaking: warn, not reject.

**Acceptance criteria:**
- `validateConfig` warns (`slog.Warn`) when `docker.image` uses tag-only reference without digest
- `parseImageRef` returns a structured type indicating whether digest is present
- Documentation updated: recommend digest-pinned references for production
- No breaking change: tag-only references still work (warn, not reject)
- Unit tests: image with digest passes, tag-only triggers warning

**Panel consensus:** 6/6. Warn-not-reject per ARCH-2 nuance adopted.

---

### SEC-09: Add security headers to embedded SPA

**Package:** `panel/api` | **Size:** XS | **Release:** v1.1.0 | **Audit:** I3

panel/api/server.go serves the SPA via `http.FileServerFS` with no header middleware. Wrap with middleware setting security headers.

**Acceptance criteria:**
- Middleware wraps SPA file server adding: `Content-Security-Policy`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`
- CSP allows inline scripts and styles (`unsafe-inline`) but blocks external sources
- Security headers do not break existing SPA functionality
- Unit test verifies all headers present on `GET /`

**Panel consensus:** Unanimous 6/6.

---

### SEC-10: Use yaml.v3 encoder for config.yaml rendering

**Package:** `internal/config` | **Size:** S | **Release:** v1.1.0 | **Audit:** I4

internal/config/inject.go `renderConfigYAML` uses `fmt.Fprintf` to construct YAML. Replace with `yaml.v3` Marshal. Preserve the `# Managed by fox-control` comment header via prepend after Marshal.

**Acceptance criteria:**
- `renderConfigYAML` uses `yaml.v3` Marshal instead of `fmt.Fprintf`
- Config struct or map represents the YAML structure (not string interpolation)
- Values with YAML-special characters (colons, quotes, `#`, `{`, `}`, newlines) are correctly escaped
- Generated config.yaml parses correctly with `yaml.v3` Unmarshal (round-trip test)
- `# Managed by fox-control` comment header preserved via prepend
- Existing config injection tests pass

**Panel consensus:** Unanimous 6/6.

---

### SEC-11: URL-escape Qdrant collection name in API paths

**Package:** `data-plane/qdrant` | **Size:** XS | **Release:** v1.1.0 | **Audit:** I5

data-plane/qdrant/client.go constructs URLs via `fmt.Sprintf` with unescaped collection names at lines 73, 101, 126, 151. Apply `net/url.PathEscape` to the collection name in each `Sprintf` call.

**Acceptance criteria:**
- All `fmt.Sprintf` calls that interpolate collection name use `net/url.PathEscape`
- Collection names with slashes, spaces, and query-string characters produce correct API URLs
- Unit test: collection name with `/`, `?`, `#`, space characters
- Existing Qdrant client tests pass

**Panel consensus:** Unanimous 6/6.

---

### OPS-01: Implement port reclamation on instance destroy

**Package:** `internal/provisioner` | **Size:** S | **Release:** v1.1.0

Verify whether `allocatePort` correctly queries the registry inside the lock. Code analysis suggests the limitation doc may be stale and the port reclamation already works correctly. Either fix the allocation logic or update LIMITATIONS.md and add a regression test.

**Acceptance criteria:**
- Integration test: provision instance A on port P, destroy A, provision instance B — B can get port P
- `allocatePort` correctly returns freed ports without requiring restart
- LIMITATIONS.md entry removed or updated
- No port leak under concurrent provision/destroy cycles (test with `-race`)
- Benchmark: port allocation remains O(1) amortized at 100 instances

**Panel consensus:** Unanimous 6/6.

---

### OPS-02: Persist event log to SQLite with configurable retention

**Package:** `internal/events` | **Size:** M | **Release:** v1.1.0

Add SQLite-backed persistent event store alongside the ring buffer. Ring buffer stays for real-time SSE fan-out. On `Emit`, also `INSERT` into SQLite. Add configurable retention with daily pruning. Add CLI subcommand for querying persistent events.

**Acceptance criteria:**
- Events persisted to `data_root/events.db` on every `Emit` call
- Ring buffer continues to serve SSE with no performance regression (benchmark Emit before/after)
- Retention pruning removes events older than configured days (default 7, range 1–365)
- CLI `fox-control events` subcommand lists events with `--since`, `--instance`, `--type` filters
- Restart preserves event history
- Config: `control.event_retention_days` (default 7)
- SQLite schema: `id INTEGER PRIMARY KEY, type TEXT, instance TEXT, message TEXT, created_at TEXT`

**Panel consensus:** Unanimous 6/6. Dissent on retention default (7 vs 30 days) resolved: 7 days.

---

### OPS-03: Add Prometheus metrics endpoint (/metrics)

**Package:** `panel/api` | **Size:** M | **Release:** v1.2.0

Add `/metrics` endpoint using `prometheus/client_golang`. Expose key operational metrics. Endpoint is unauthenticated per Prometheus conventions (operators must restrict access at the network layer if the panel is exposed beyond localhost).

**Acceptance criteria:**
- `GET /metrics` returns valid Prometheus exposition format
- `/metrics` does not require Bearer auth
- Metrics: instance gauge (by status), health gauge, provision duration histogram, API request histogram/counter, SSE connections gauge
- Metric names follow Prometheus naming conventions (`snake_case`, `_total` for counters, `_seconds` for histograms)
- Config flag `control.metrics_enabled` (default true) to disable
- Benchmark shows <1% overhead on API request latency

**Panel consensus:** 6/6. Dissent on auth resolved: unauthenticated.

---

### OPS-04: Add built-in TLS termination

**Package:** `cmd/fox-control` | **Size:** M | **Release:** v1.2.0

Manual TLS only for Apache scope. When `tls.cert_file` and `tls.key_file` are set, use `srv.ListenAndServeTLS`. ACME/Let's Encrypt deferred.

**Acceptance criteria:**
- Manual TLS: fox-control serves HTTPS when `tls.cert_file` and `tls.key_file` are set
- HTTP mode continues to work when no TLS config is provided (no regression)
- Config validation rejects setting both manual and ACME TLS fields simultaneously
- Data plane server also uses TLS when configured
- Integration test: start with self-signed cert, verify HTTPS handshake succeeds
- Documentation: add TLS configuration section to deployment guide

**Panel consensus:** 5/6. Dissent on ACME resolved: manual cert only for Apache scope.

---

### OPS-05: Implement SQLite backup/restore CLI commands

**Package:** `cmd/fox-control` | **Size:** S | **Release:** v1.2.0

Add `fox-control backup` and `fox-control restore` subcommands. **Use `VACUUM INTO`** for consistent snapshots — the `sqlite3_backup_init` C API is not available through the modernc.org/sqlite driver.

**Acceptance criteria:**
- `fox-control backup --output path` creates consistent backup of registry.db and sources.db
- `fox-control restore --input path` restores databases from backup
- Backup uses `VACUUM INTO` (not the C backup API, which is unavailable in modernc.org/sqlite)
- Restore refuses to overwrite if databases are locked by running server
- Backup includes events.db when present (OPS-02)
- Integration test: provision, backup, destroy, restore, verify instance is back

**Panel consensus:** 5/6.

---

### OPS-06: Add fox-control diagnostics command

**Package:** `cmd/fox-control` | **Size:** S | **Release:** v1.2.0

Add `fox-control diagnostics` command running 8+ health checks with a structured table output.

**Acceptance criteria:**
- `fox-control diagnostics` runs 8+ checks and outputs a table of name/status/detail
- Checks: config validity, Docker socket, registry integrity (`PRAGMA integrity_check`), container-registry cross-reference, Qdrant reachability, embedding service reachability, disk space, port availability
- Exit code 0 when all pass, 1 when any fail
- Checks for optional components skip when not configured
- Runs in under 10 seconds total

**Panel consensus:** 5/6.

---

### DP-01: Add retry with exponential backoff to embedding client

**Package:** `data-plane/embedding` | **Size:** S | **Release:** v1.3.0

Add configurable retry with exponential backoff and jitter to the embedding client. Retry on retryable status codes and transient network errors only.

**Acceptance criteria:**
- Retries on 429, 500, 502, 503, 504 and transient network errors
- Does not retry on 400, 401, 403 (client errors)
- Exponential backoff with jitter between retries
- Respects context cancellation during backoff wait
- Default: 3 attempts, 1s initial backoff, 10s max backoff
- Unit tests with `httptest`: 429-then-200, 503-then-200, permanent 400, context cancel during retry

**Panel consensus:** Unanimous 6/6.

---

### DP-02: Add configurable timeout to Qdrant client operations

**Package:** `data-plane/qdrant` | **Size:** S | **Release:** v1.3.0

Remove global 30s `http.Client` timeout. Use per-operation context-based deadlines with sensible defaults.

**Acceptance criteria:**
- Qdrant client constructor accepts optional timeout configuration
- Context deadline is respected without being capped by `http.Client.Timeout`
- Per-operation defaults: 5s health, 30s search, 60s upsert, 30s collection ops
- Health check times out in 5s when Qdrant is unreachable
- Unit tests verify timeout behavior for each operation type

**Panel consensus:** 6/6.

---

### DP-03: Add Qdrant health monitoring to health poller

**Package:** `panel/api` | **Size:** S | **Release:** v1.3.0 | **Deps:** DP-02

Extend `HealthPoller` to check Qdrant when data_plane is enabled. Surface in `/healthz` response and emit events on health transitions.

**Acceptance criteria:**
- `/healthz` response includes Qdrant health status when data_plane is enabled
- Health poller checks Qdrant on each poll cycle
- Event emitted on Qdrant health state transition (healthy → unhealthy and back)
- Panel SPA can display Qdrant health (API contract; SPA update is separate ticket)
- No regression when data_plane is disabled (Qdrant check skipped)

**Panel consensus:** 5/6.

---

### DP-04: Implement incremental source re-ingestion with content hashing

**Package:** `data-plane/ingestion` | **Size:** M | **Release:** v1.3.0 | **Deps:** DP-01

Add content hash tracking per document. On re-ingest, compare hashes to skip unchanged documents, re-embed changed ones, and delete points for removed documents. Note: file connector hashes file content, REST connector tracks `doc.ID` + content hash — these are architecturally different implementations.

**Acceptance criteria:**
- Re-ingesting unchanged source skips embedding and upsert (verified by counting API calls)
- Modified documents are re-embedded and re-upserted
- Deleted documents have their Qdrant points removed
- New `document_tracking` table in sources.db stores per-document content hashes (SHA-256)
- Qdrant client has `DeleteByFilter` method for point cleanup
- Both file and REST connectors implement incremental logic
- Benchmark: re-ingest of unchanged 100-document source completes in <1s

**Panel consensus:** 4/6.

---

### DP-05: Add data plane WriteTimeout and request body limits

**Package:** `data-plane/server` | **Size:** XS | **Release:** v1.1.0

Same class of issue as SEC-04 but on the data plane server. Add `WriteTimeout: 300s` (longer because ingestion responses take minutes) and `MaxBytesReader` to all body-reading handlers.

**Acceptance criteria:**
- Data plane HTTP server has `WriteTimeout` set (300s)
- All handlers that read request bodies use `http.MaxBytesReader`
- WriteTimeout is long enough for ingestion operations
- Existing tests pass

**Panel consensus:** 3/6.

---

### DP-06: Add Qdrant upsert batching for large ingestions

**Package:** `data-plane/qdrant` | **Size:** S | **Release:** v1.3.0

Batch upserts into configurable chunks to prevent oversized requests to Qdrant.

**Acceptance criteria:**
- Qdrant `Upsert` splits large point arrays into configurable batch sizes (default 100 points)
- Batches sent sequentially with error handling per batch
- Partial failure reports which batch failed
- Unit test: 500 points split into 5 batches of 100
- Benchmark: 10K point upsert completes without OOM or timeout

**Panel consensus:** 3/6.

---

### DP-07: Add source deletion cascade to remove Qdrant points

**Package:** `data-plane/server` | **Size:** S | **Release:** v1.3.0

`handleAdminDeleteSource` deletes from SQLite but leaves all ingested points in Qdrant. Cascade deletion to Qdrant points filtered by `source_id`.

**Acceptance criteria:**
- `DELETE /admin/sources/{id}` also deletes all Qdrant points for that source
- Deletion uses Qdrant filter by `source_id` metadata field
- If Qdrant deletion fails, source deletion is rolled back (or error returned)
- Unit test: create source, ingest, delete source, verify Qdrant points are gone
- Existing source deletion tests pass

**Panel consensus:** Solo proposal, included for data hygiene (orphaned vectors degrade RAG quality).

---

### DP-08: Add embedding dimension validation on startup

**Package:** `data-plane/server` | **Size:** S | **Release:** v1.3.0

Data plane config has `vector_size` and embedding model config but these are never cross-validated. Send a test embedding on startup and validate dimension.

**Acceptance criteria:**
- Data plane startup sends a test embedding request to validate dimension matches config `vector_size`
- Dimension mismatch produces a clear fatal error with both expected and actual dimensions
- Validation runs before any collection creation or ingestion
- Skip validation if embedding service is unreachable (warn, do not block startup)
- Unit test: matching dimensions pass, mismatched dimensions fail with descriptive error

**Panel consensus:** Solo proposal, included for data integrity (silent dimension mismatch produces corrupt vectors).

---

### DP-09: Add query result score threshold filtering

**Package:** `data-plane/server` | **Size:** XS | **Release:** v1.3.0

`handleQuery` returns all results up to `top_k` regardless of cosine similarity score. Add optional `min_score` parameter.

**Acceptance criteria:**
- `handleQuery` accepts optional `min_score` parameter (float, 0.0–1.0)
- Results below `min_score` excluded from response
- Default behavior (no `min_score`) unchanged
- Config: `data_plane.default_min_score` (default 0.0)
- Unit test: results with scores [0.9, 0.5, 0.1] filtered at 0.3 returns [0.9, 0.5]

**Panel consensus:** Solo proposal, included for XS effort with direct RAG quality impact.

---

### INT-01: Implement webhook forwarding (POST-on-event)

**Package:** `internal/events` | **Size:** M | **Release:** v1.2.0 | **Deps:** OPS-02

Simple POST-on-event per Apache boundaries. No retry, dead-letter, or transforms (those are Enterprise). Include `X-Fox-Timestamp` header in HMAC input for replay protection.

**Acceptance criteria:**
- Config: `control.webhooks` array with `url`, `events` filter, and `secret` fields
- Events matching filter trigger POST to configured URL
- `X-Fox-Signature` header (HMAC-SHA256 of body with shared secret)
- `X-Fox-Event` header with event type; `X-Fox-Timestamp` header included in HMAC input
- 5s delivery timeout, no retry on failure (Apache scope)
- Failed deliveries logged as events (not recursive)
- Rate limit: max 10 deliveries per second per endpoint
- Unit test with `httptest`: delivery, timeout, HMAC verification

**Panel consensus:** 6/6.

---

### INT-02: Add structured JSON logging option

**Package:** `cmd/fox-control` | **Size:** S | **Release:** v1.2.0

Add config for JSON output (`slog.NewJSONHandler`) and log level control. Apply configured handler at startup before any initialization.

**Acceptance criteria:**
- `control.log_format = json` produces structured JSON log lines
- `control.log_format = text` produces current text format (default, no regression)
- `control.log_level` controls minimum log level (debug, info, warn, error; default info)
- JSON output includes timestamp, level, message, and all structured fields
- Handler set before registry open (migration logs also structured)
- Config validation rejects invalid `log_format` and `log_level` values

**Panel consensus:** 6/6.

---

### PLAT-01: Add runtime validation of skillset-declared tools

**Package:** `skillsets` | **Size:** M | **Release:** v1.3.0

Add `ValidateAgainstImage` to `skillsets/manifest.go`. Convention-based discovery (expect tools.json in known location). Validation is advisory (warn, not block).

**Acceptance criteria:**
- `ValidateAgainstImage` checks declared tools against instance tool manifest
- Validation runs at provision time (before container start)
- Mismatched tools produce a warning event (do not block provisioning)
- Convention-based discovery: check for tools.json in image or container
- Unit tests: matching tools pass, missing tool warns, extra tool in image is fine
- Validation skipped when image does not publish a tool manifest (graceful degradation)

**Panel consensus:** Unanimous 6/6.

---

### PLAT-02: Implement graceful shutdown with in-flight request draining

**Package:** `cmd/fox-control` | **Size:** S | **Release:** v1.1.0

main.go handles SIGTERM and calls `srv.Shutdown` with 5s timeout but does not drain in-flight provisioning. Track in-flight operations with `sync.WaitGroup`, drain SSE clients, and wait up to configurable timeout.

**Acceptance criteria:**
- SIGTERM/SIGINT triggers graceful shutdown sequence
- In-flight provisioning operations complete before server stops (up to drain timeout)
- SSE clients receive a close event before disconnect
- Drain timeout configurable (default 30s, range 5–300)
- Log messages indicate shutdown progress
- Test: start provision, send SIGTERM, verify provision completes and server exits cleanly

**Panel consensus:** 5/6.

---

### PLAT-03: Add health-check-based auto-restart for unhealthy instances

**Package:** `panel/api` | **Size:** M | **Release:** v1.3.0

Add configurable auto-recovery. Default off. Include cooldown to prevent restart storms and per-instance opt-out.

**Acceptance criteria:**
- Config: `control.auto_restart_enabled` (default false), `control.auto_restart_threshold` (consecutive unhealthy checks, default 3), `control.auto_restart_cooldown` (seconds, default 300)
- After threshold consecutive unhealthy checks, poller triggers destroy + re-provision
- Auto-restart emits events (`instance.auto_restart` with reason)
- Cooldown prevents restart storms
- Instance config (skillset, role, env) preserved across restart
- Manual opt-out per instance via instance metadata flag

**Panel consensus:** 4/6.

---

### PLAT-04: Add registry database migration versioning

**Package:** `internal/registry` | **Size:** S | **Release:** v1.1.0

registry.go `migrate()` uses `CREATE TABLE IF NOT EXISTS` and `ALTER TABLE` with error swallowing. Add a simple forward-only migration system with version tracking.

**Acceptance criteria:**
- Schema version tracked in a `schema_version` table (single row, version `INTEGER`)
- Each migration is a numbered function (1, 2, 3...) that runs exactly once
- `Migrate` runs all pending migrations in order within a transaction
- Down-migration not required (forward-only)
- Migration 1 is the current `CREATE TABLE IF NOT EXISTS` (existing schema)
- Current `ALTER TABLE` with error swallowing converted to migration 2 (three columns: skillset_name, principal_role, query_token)
- Test: start with empty DB, migrate to latest, verify schema

**Panel consensus:** 2/6 explicit, but implicit dependency from multiple agents.

---

### CONF-01: Add data plane round-trip conformance checks

**Package:** `conformance` | **Size:** M | **Release:** v1.3.0

Add data plane conformance checks for source CRUD, ingestion, and query. Also fix plugin conformance to accept interface, not concrete `docker.Plugin`.

**Acceptance criteria:**
- New `conformance/dataplane` package with check functions
- Checks: source CRUD, file ingestion round-trip, REST ingestion round-trip, query returns results, embedding health, Qdrant collection lifecycle
- Each check is independent and reports pass/fail/skip with detail
- Suite runner matches existing runtime/plugin conformance pattern
- At least 10 checks covering the full data plane surface
- `conformance/plugin/checks.go` refactored to accept interface (not concrete `docker.Plugin`)

**Panel consensus:** Unanimous 6/6.

---

### CONF-02: Add security-focused conformance checks

**Package:** `conformance/runtime` | **Size:** S | **Release:** v1.3.0 | **Deps:** SEC-09

Add conformance checks verifying security properties shipped in SEC-XX tickets.

**Acceptance criteria:**
- Checks verify: security headers present on SPA responses, authenticated endpoints reject missing/invalid bearer token
- Each check follows existing conformance check pattern
- At least 5 security-focused checks
- Suite runner includes security checks in runtime conformance

**Panel consensus:** 2/6.

---

### REL-01: Add govulncheck to CI pipeline

**Package:** `build/ci` | **Size:** S | **Release:** v1.2.0

Add govulncheck to CI workflow for continuous vulnerability scanning with call-graph analysis.

**Acceptance criteria:**
- GitHub Actions workflow runs govulncheck on every PR and push to main
- Fails the build on any reachable vulnerability (call-graph analysis, not just go.mod)
- Uses pinned govulncheck version via `go install golang.org/x/vuln/cmd/govulncheck@vX.Y.Z`
- Results are machine-parseable (JSON output)
- Runs after `go build` succeeds (needs compiled binary for call-graph analysis)
- Documentation: add govulncheck to local quality gate instructions

**Panel consensus:** 5/6.

---

### REL-02: Add automated dependency update workflow

**Package:** `build/ci` | **Size:** S | **Release:** v1.2.0

Configure Dependabot or Renovate for Go modules, GitHub Actions, and Docker base images.

**Acceptance criteria:**
- Dependabot or Renovate configuration file in repository root
- Go module updates checked weekly with auto-PR
- GitHub Actions updates checked weekly with auto-PR
- Docker base image updates checked weekly
- Auto-merge for patch updates that pass CI (optional, configurable)
- PR labels for dependency update PRs

**Panel consensus:** 5/6.

---

### REL-03: Add release health monitoring workflow

**Package:** `build/ci` | **Size:** S | **Release:** v1.3.0 | **Deps:** REL-01

Scheduled workflow verifying the latest release remains healthy: artifacts exist, signatures verify, install paths work.

**Acceptance criteria:**
- Scheduled GitHub Actions workflow runs daily at 6 AM UTC
- Checks: latest release tag exists, container image pullable, cosign signature verifies, SBOM attestation verifies, govulncheck on release binary
- Failures create a GitHub issue with detailed failure report
- No false positives from transient network issues (retry with backoff)

**Panel consensus:** 2/6.

---

### DOC-01: Write operator handbook

**Package:** `docs` | **Size:** L | **Release:** v1.4.0 | **Deps:** OPS-02, OPS-03, OPS-05

Comprehensive operator handbook covering production deployment, day-2 operations, backup/restore, monitoring, capacity planning, secret rotation, and troubleshooting.

**Acceptance criteria:**
- `docs/OPERATOR-HANDBOOK.md` with sections: production checklist, backup/restore, monitoring setup, capacity planning, secret rotation, troubleshooting
- Production checklist: pre-deploy verification steps with commands
- Backup section references `fox-control backup` command (OPS-05)
- Monitoring section includes Prometheus scrape config examples (OPS-03)
- Secret rotation covers admin_secret, SSE signing key, query tokens
- Troubleshooting section covers at least 15 failure scenarios with diagnosis commands
- All commands are copy-pasteable and tested against current version

**Panel consensus:** Unanimous 6/6.

---

### DOC-02: Write developer and contributor handbook

**Package:** `docs` | **Size:** M | **Release:** v1.4.0

Expand CONTRIBUTING.md and create DEVELOPER.md covering architecture, package layout, plugin development, connector development, ADR authoring, and PR workflow.

**Acceptance criteria:**
- `docs/DEVELOPER.md`: architecture overview, package layout, development setup, testing guide, ADR authoring guide
- Plugin development guide: how to implement `DeploymentPlugin` interface with example
- Ingestion connector development guide: how to add a new connector with example
- CONTRIBUTING.md expanded with PR workflow, commit conventions, review process
- Architecture diagram (text-based) showing control plane, data plane, plugin interface
- All code examples compile and tests pass

**Panel consensus:** Unanimous 6/6.

---

### DOC-03: Add API reference documentation

**Package:** `docs` | **Size:** M | **Release:** v1.4.0

Create API reference for both panel API and data plane API, preferably as OpenAPI 3.0 spec.

**Acceptance criteria:**
- API reference for panel API: all endpoints, methods, request/response bodies, auth requirements, status codes
- API reference for data plane API: all endpoints, methods, request/response bodies, auth requirements
- OpenAPI 3.0 spec file (openapi.yaml) or equivalent structured format
- Integrated into mkdocs-material docs site
- All examples tested against current version (curl commands that work)

**Panel consensus:** 3/6.

---

### DOC-04: Add example deployment configurations

**Package:** `docs` | **Size:** S | **Release:** v1.4.0

Add example configurations demonstrating common deployment patterns.

**Acceptance criteria:**
- `examples/` directory with at least 3 deployment configurations: minimal (single binary), production (Docker Compose with TLS), air-gapped (pre-pulled images)
- Each example includes a README with step-by-step instructions
- Each example is testable (`docker compose up` succeeds, binary starts)
- Examples reference current version and images

**Panel consensus:** Solo proposal, included for adoption impact.

---

### PERF-01: Optimize ValidQueryToken with indexed lookup

**Package:** `internal/registry` | **Size:** XS | **Release:** v1.2.0

`ValidQueryToken` queries ALL query tokens and iterates in Go with `subtle.ConstantTimeCompare`. This is O(N). Use a hash-then-lookup pattern: store SHA-256 of each token in an indexed column, query by hash, then constant-time compare the raw token for the matched row.

**Acceptance criteria:**
- New indexed column `query_token_hash` on instances table storing `SHA-256(query_token)`
- `ValidQueryToken` queries by hash (indexed, O(1)), then compares raw token with `subtle.ConstantTimeCompare`
- Migration backfills `query_token_hash` for existing tokens
- `UpdateQueryToken` and `Create` also set `query_token_hash`
- Benchmark: ValidQueryToken at 1000 instances is <100μs
- Constant-time comparison preserved at the Go layer

**Panel consensus:** 2/6.

---

### PERF-02: Batch embedding requests in ingestion connectors

**Package:** `data-plane/ingestion` | **Size:** M | **Release:** v1.3.0 | **Deps:** DP-01

Both connectors send all chunks in one embedding request. Batch into configurable sizes with HTTP connection pooling.

**Acceptance criteria:**
- File and REST connectors split chunk arrays into configurable batch sizes (default 256 chunks)
- Batches sent sequentially with error handling per batch
- Partial batch failure does not lose successfully embedded chunks
- Benchmark: 50MB file ingestion completes without provider timeout or OOM
- Connection pooling configured on embedding HTTP client

**Panel consensus:** 4/6.

---

### PANEL-01: Add instance health history and uptime indicator

**Package:** `panel/spa` | **Size:** S | **Release:** v1.4.0 | **Deps:** OPS-02

With persistent events (OPS-02), show health history timeline and uptime duration.

**Acceptance criteria:**
- Instance detail view shows health history timeline (last 24h from persistent store)
- Instance card shows uptime duration (time since last healthy transition)
- Panel queries persistent event log for health history data
- Graceful degradation when persistent events not available
- Works on mobile viewport

**Panel consensus:** 3/6.

---

### PANEL-02: Add instance resource usage display

**Package:** `panel/api` | **Size:** M | **Release:** v1.4.0

Add API endpoint and panel visualization for CPU, memory, and network I/O per instance.

**Acceptance criteria:**
- New API endpoint returning per-instance resource stats (CPU %, memory bytes/limit, network I/O)
- Docker plugin extended with `ContainerStats` method using Docker API
- Instance detail view displays resource usage with simple bar/gauge visualization
- Stats refresh on panel poll cycle
- Graceful degradation: stats unavailable for stopped instances

**Panel consensus:** 2/6.

---

### PANEL-03: Prevent race condition in handleCreate duplicate check

**Package:** `panel/api` | **Size:** XS | **Release:** v1.1.0

`handleCreate` has non-atomic check-then-act. The provisioner's mutex prevents duplicate containers, but the HTTP handler returns 201 before the goroutine runs — a concurrent request can also get 201. Fix: atomic check-and-insert in the registry to prevent the false-positive 201.

**Acceptance criteria:**
- `handleCreate` uses atomic check-and-insert (not check-then-insert with goroutine gap)
- Concurrent create requests for same ID: exactly one succeeds, others get 409 Conflict
- Unit test with `sync.WaitGroup`: 10 concurrent creates for same ID, exactly 1 success
- Existing create tests pass

**Panel consensus:** 3/6.

---

### PANEL-04: Add panel i18n contribution workflow

**Package:** `panel/spa` | **Size:** S | **Release:** v1.4.0

Extract translation strings from inline JS into structured files. Add contribution workflow.

**Acceptance criteria:**
- i18n strings extracted into structured format (JSON per language)
- Translation contribution guide in CONTRIBUTING.md
- At least one additional language beyond en/es
- Automated validation that all translation keys are present in all language files
- Panel language switcher works with new languages

**Panel consensus:** 3/6.

---

### PANEL-05: Add instance create form with skillset/role selection

**Package:** `panel/spa` | **Size:** S | **Release:** v1.4.0

The API supports `skillset_path` and `role` but the panel only shows an instance ID field. SPA-only change.

**Acceptance criteria:**
- Provision modal includes `skillset_path` and `role` fields
- Skillset dropdown populated from `GET /api/skillsets` endpoint
- Role field is a text input with common role suggestions
- API already supports these fields (handlers.go line 42–44) — SPA-only
- Existing provision flow continues to work with just instance ID

**Panel consensus:** 2/6.

---

### POLISH-01: Add rate limiting to panel API endpoints

**Package:** `panel/api` | **Size:** S | **Release:** v1.2.0

Token bucket rate limiter on all API endpoints. Exempt `/healthz` and `/metrics`.

**Acceptance criteria:**
- Token bucket rate limiter on all API endpoints
- Default: 100 requests/minute globally, 10 provisions/minute
- 429 Too Many Requests response with `Retry-After` header
- Rate limits configurable via config
- Exempt: `/healthz`, `/metrics`
- Unit test: burst of requests, verify 429 after limit

**Panel consensus:** 4/6.

---

### POLISH-02: Add CLI output formatting: table, JSON, quiet modes

**Package:** `cmd/fox-control` | **Size:** S | **Release:** v1.2.0

Shared output formatter with table, JSON, and quiet modes via `--output` flag.

**Acceptance criteria:**
- Global `--output` flag with values: `table` (default), `json`, `quiet`
- Table mode: current tabwriter behavior (no regression)
- JSON mode: structured JSON output for all list/get commands
- Quiet mode: minimal output (IDs only) for scripting
- Applied to: list, events, diagnostics, backup commands
- Unit test: same data renders correctly in all three modes

**Panel consensus:** Solo proposal, included for operator scripting.

---

## Dependency Graph

```
SEC-04 ─┐
        └─ DP-05

OPS-02 ──┬─ INT-01
         ├─ PANEL-01
         └─ DOC-01

OPS-03 ──── DOC-01

OPS-05 ──── DOC-01

SEC-09 ──── CONF-02

DP-01 ───┬─ DP-04
         └─ PERF-02

DP-02 ──── DP-03

REL-01 ──── REL-03
```

All other tickets have no dependencies and can be parallelized within their release.
