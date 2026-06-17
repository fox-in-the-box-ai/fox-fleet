# Fox Fleet Developer Handbook

Reference guide for contributors to the Fox Fleet management plane.

**Module:** `github.com/fox-in-the-box-ai/fox-fleet`
**Go version:** 1.25.0
**Binary:** `fox-control`

---

## 1. Architecture Overview

```
                         fox-control binary
                               |
          +--------------------+--------------------+
          |                    |                    |
    CLI commands         Panel (HTTP)         Data Plane (HTTP)
   provision, list,      :9090                :9091
   destroy, rollout,       |                    |
   conformance, sec       /api/* + SPA         /v1/*
          |                |                    |
          |         +------+------+      +------+------+
          |         |      |      |      |      |      |
          |      Health  Event  Session  Query  Admin  Source
          |      Poller  Log    Token   Handler Handler CRUD
          |         |      |               |
          +---------+------+--------+      |
                    |               |      |
              +-----------+   +-----------+|
              | Registry  |   | Provisioner||
              | (SQLite)  |   |            ||
              +-----------+   +-----+------+|
                                    |       |
                              +-----+-----+ |
                              |  Plugin   | |
                              | Interface | |
                              +-----+-----+ |
                                    |       |
                              +-----+-----+ +---+-------+---+
                              |  Docker   | |Embedding|Qdrant|
                              |  Plugin   | | Client  |Client|
                              +-----+-----+ +----+----+--+--+
                                    |             |       |
                              +-----+-----+ +----+----+--+--+
                              |  Docker   | |Ollama / |Qdrant|
                              | Containers| |OpenAI   | DB   |
                              | (Fox      | |compat.  |      |
                              | instances)| |endpoint |      |
                              +-----------+ +---------+------+
```

**Two listen ports:**

| Port | Service | Purpose |
|------|---------|---------|
| 9090 (default) | Panel API + embedded SPA | Instance management, events, skillsets, metrics |
| 9091 (default) | Data plane API | Vector search, source CRUD, ingestion |

The panel and data plane share the same process. The panel manages Fox instance containers through the plugin interface; the data plane manages knowledge ingestion and retrieval through Qdrant.

---

## 2. Package Layout

### CLI layer

| Package | Responsibility | Key types |
|---------|---------------|-----------|
| `cmd/fox-control/` | CLI entry point, TOML config parsing, cobra subcommands | `Config`, `LoadConfig()`, `newServeCmd()`, `newRolloutCmd()` |

Subcommands: `serve`, `provision`, `destroy`, `list`, `rollout`, `version`, `conformance run`, `conformance plugin`, `sec rotate-sse-key`, `sec rotate-query-token`, `verify`, `backup`, `restore`, `diagnostics`, `generate-secret`.

### Internal services

| Package | Responsibility | Key types |
|---------|---------------|-----------|
| `internal/config/` | Config file injection into instance data directories. Renders `hermes.env`, `config.yaml`, `settings.json`, `tools.json`. | `InjectParams`, `Inject()`, `ValidateSecrets()` |
| `internal/registry/` | SQLite-backed instance registry. CRUD, port allocation, schema migrations, signing key management, query token storage. | `Registry`, `Instance`, `ErrNotFound` |
| `internal/provisioner/` | Orchestrates provisioning: cap check, port alloc, config injection, plugin deploy, health wait, rollback on failure. | `Provisioner` (interface), `Request`, `Instance`, `Options` |
| `internal/events/` | Event log with ring buffer, SQLite persistence, pub/sub channels, and webhook dispatch. | `Log`, `Event`, `Store`, `WebhookDispatcher` |
| `internal/sessiontoken/` | HMAC-SHA256 session token signing and verification with purpose bytes and expiry. | `Signer`, `PurposeSSE` |
| `internal/safedialer/` | SSRF-safe `net.Dialer` that blocks connections to private/loopback/link-local IPs. | `New()` returns `*net.Dialer` |
| `internal/output/` | CLI output formatting supporting table, JSON, and quiet modes. | `Writer`, `Format`, `ParseFormat()` |

### Plugin system

| Package | Responsibility | Key types |
|---------|---------------|-----------|
| `plugins/` | `DeploymentPlugin` interface and shared types. | `DeploymentPlugin`, `ProvisionRequest`, `InstanceConfig`, `HealthStatus`, `ContainerStats`, `ImageRef`, `LogOpts` |
| `plugins/docker/` | Docker implementation of `DeploymentPlugin`. Manages container lifecycle, health probes, stats collection. | `Plugin`, `New()`, `NewWithClient()` |

### Panel

| Package | Responsibility | Key types |
|---------|---------------|-----------|
| `panel/api/` | Panel HTTP server, route registration, auth middleware, rate limiting, health poller, metrics, SSE event stream. | `Server`, `Deps`, `HealthPoller`, `QdrantHealthChecker` |
| `panel/spa/` | Embedded SPA assets via `//go:embed static`. | `Static` (embed.FS) |

### Data plane

| Package | Responsibility | Key types |
|---------|---------------|-----------|
| `data-plane/` | Package doc and entry point for the data plane server. | — |
| `data-plane/server/` | Data plane HTTP handlers: query, admin source CRUD, ingestion triggers, health/readyz. | `Server`, `Config`, `QueryTokenValidator` |
| `data-plane/chunker/` | Rune-aware text chunking with configurable size and overlap. | `Split()`, `Chunk`, `Options` |
| `data-plane/embedding/` | OpenAI-compatible embedding client with retry and exponential backoff. | `Client`, `Config`, `Embed()` |
| `data-plane/ingestion/` | Ingestion connector interface and shared types. | `Plugin` (interface), `SourceConfig`, `IngestResult`, `SourceStatus` |
| `data-plane/ingestion/file/` | File ingestion connector. Reads `.txt`, `.csv`, `.md` from an allowed directory, chunks, embeds, upserts. Tracks content hashes for incremental re-ingestion. | `Connector`, `DocTracker` |
| `data-plane/ingestion/rest/` | REST API ingestion connector. Paginated fetch from a remote JSON endpoint with SSRF protection via `safedialer`. | `Connector` |
| `data-plane/qdrant/` | Qdrant vector DB HTTP client. Collection management, upsert (batched), search, delete by filter. | `Client`, `Point`, `SearchRequest`, `SearchResult`, `Filter` |
| `data-plane/source/` | SQLite-backed source metadata registry. Tracks source status, doc/chunk counts, document content hashes. | `Registry`, `Source`, `ErrNotFound` |

### Operations

| Package | Responsibility | Key types |
|---------|---------------|-----------|
| `rollout/` | Fleet-wide rolling update orchestration. Sequential per-instance rollout with health-gated promotion and automatic rollback on failure. | `Orchestrator`, `Options`, `Report`, `InstanceResult`, `ResultStatus` |
| `skillsets/` | Skillset manifest parsing, semver validation, tool manifest cross-validation, Hermes config translation. | `Manifest`, `Validate()`, `ValidateAgainstManifest()`, `ToolValidationResult` |
| `conformance/` | Conformance test suites for runtime behavior, plugin contracts, and data plane API. | — |
| `conformance/runtime/` | Runtime conformance: 24 checks covering standalone/managed boot, auth, health, SSE, security headers, path traversal. | `Suite`, `Run()` |
| `conformance/plugin/` | Plugin conformance: 8 checks covering provision/health/configure/rollout/rollback/destroy lifecycle, idempotency, error handling. | `Suite`, `Run()` |
| `conformance/dataplane/` | Data plane conformance: 10 checks covering health, readiness, admin auth, source CRUD, query auth, and content type. | `Suite`, `Run()` |

---

## 3. Plugin Development Guide

To add a new deployment target (e.g., Kubernetes, Podman), implement the `DeploymentPlugin` interface from `plugins/plugin.go`:

```go
type DeploymentPlugin interface {
    Provision(ctx context.Context, req ProvisionRequest) error
    HealthCheck(ctx context.Context, instanceID string) (HealthStatus, error)
    Configure(ctx context.Context, instanceID string, cfg InstanceConfig) error
    Rollout(ctx context.Context, instanceID string, target ImageRef) error
    Rollback(ctx context.Context, instanceID string, previous ImageRef) error
    Destroy(ctx context.Context, instanceID string) error
    Logs(ctx context.Context, instanceID string, opts LogOpts) (io.ReadCloser, error)
    Restart(ctx context.Context, instanceID string) error
    Stats(ctx context.Context, instanceID string) (ContainerStats, error)
}
```

### Method contracts

| Method | Expected behavior |
|--------|------------------|
| `Provision` | Create and start a new instance. Bind the host port from `req.Port` to the container's internal port (8080). Mount `req.DataDir` as `/data`. Set environment variables from `req.Config`. Block until the instance is healthy or the context expires. Return `nil` on success. |
| `HealthCheck` | Probe the instance and return its current health/readiness state. Must not block for longer than a few seconds. Return the status even if unhealthy (error only for infrastructure failures like "container not found"). |
| `Configure` | Apply runtime configuration changes to a running instance. May be a no-op if the plugin doesn't support hot-reconfiguration. |
| `Rollout` | Replace the instance's image with `target`. Pull the new image, stop the old container, create and start a new one with the same config and port bindings, wait for health. If health fails, the caller (rollout orchestrator) will invoke `Rollback`. |
| `Rollback` | Revert to `previous` image. Semantically identical to `Rollout` with the old image ref. The Docker plugin implements this as `return p.Rollout(ctx, instanceID, previous)`. |
| `Destroy` | Stop and remove the instance. Must be idempotent -- destroying a non-existent instance returns `nil`. The caller handles data directory cleanup separately. |
| `Logs` | Return a stream of container logs. Respect `opts.Tail` (last N lines) and `opts.Follow` (streaming). The caller closes the returned `io.ReadCloser`. |
| `Restart` | Stop and restart the instance in place (same image, same config). |
| `Stats` | Return point-in-time resource usage: CPU percentage, memory used/limit, network rx/tx bytes. |

### Compile-time interface check

Add this to your plugin package to catch missing methods at compile time:

```go
var _ plugins.DeploymentPlugin = (*YourPlugin)(nil)
```

### Conformance suite

Run the plugin conformance suite against your implementation:

```bash
go run ./cmd/fox-control conformance plugin --image <fox-image>
```

The suite (`conformance/plugin/`) runs 8 checks:

1. Provision a test instance
2. HealthCheck returns healthy after provision
3. Configure doesn't error on a running instance
4. Rollout to the same image succeeds
5. Rollback to the same image succeeds
6. Destroy removes the instance
7. Provision is idempotent (second call with same ID succeeds)
8. HealthCheck on a non-existent instance returns an error

### Plugin registration

Bind your plugin at the composition root in `cmd/fox-control/main.go`. The current Docker plugin is created in `openRegistryAndPlugin()`. Replace or extend that function to select the plugin based on configuration.

---

## 4. Data Plane Connector Development

To add a new ingestion connector (beyond `file` and `rest`), implement the `ingestion.Plugin` interface from `data-plane/ingestion/plugin.go`:

```go
type Plugin interface {
    Connect(ctx context.Context, cfg SourceConfig) error
    Ingest(ctx context.Context, sourceID string) (*IngestResult, error)
    Status(ctx context.Context, sourceID string) (*SourceStatus, error)
    Disconnect(ctx context.Context, sourceID string) error
}
```

### Method contracts

| Method | Expected behavior |
|--------|------------------|
| `Connect` | Validate and store the source configuration. Do not start ingestion. Return an error if the config is invalid (missing required fields, inaccessible endpoint, path outside allowed directory). |
| `Ingest` | Run the full ingestion pipeline for the given source: fetch documents, chunk text, embed via the embedding client, upsert vectors into Qdrant. Return an `IngestResult` with counts and any per-document errors. |
| `Status` | Return the current connection state and counts for the source. |
| `Disconnect` | Remove the source from the connector's internal state. Does not delete ingested data from Qdrant. |

### Ingestion pipeline

Every connector follows the same pipeline:

1. **Fetch** -- Retrieve raw documents from the source (files, API, database, etc.)
2. **Chunk** -- Split document text using `chunker.Split(text, chunker.Options{})`. Default chunk size is 512 runes with 64-rune overlap.
3. **Embed** -- Convert chunks to vectors via `embedding.Client.Embed(ctx, texts)`. Batch up to 256 texts per call.
4. **Upsert** -- Store vectors in Qdrant via `qdrant.Client.Upsert(ctx, collection, points)`. The client auto-batches in groups of 100.

Each point must include a deterministic ID (SHA-256 of `sourceID:docID:chunkIndex`, truncated to 16 bytes hex) and a payload with at minimum:

```go
map[string]any{
    "text":      chunk.Text,
    "source_id": sourceID,
    "doc_id":    docID,
    "chunk_idx": chunk.Index,
}
```

### SSRF protection

If your connector makes outbound HTTP requests, use `safedialer.New()` as the transport's `DialContext` to block connections to private IP ranges. See the REST connector for an example:

```go
http: &http.Client{
    Timeout:   60 * time.Second,
    Transport: &http.Transport{DialContext: safedialer.New().DialContext},
},
```

### Wiring into the server

1. Add your connector to `data-plane/server/server.go` in the `New()` constructor.
2. Add a case for your source type in `handleAdminIngestSource()`.
3. Add your type to the validation in `handleAdminCreateSource()` (the `req.Type` check).

---

## 5. Testing

### Running tests

```bash
# Full suite with race detection and shuffled order
go test -count=1 -race -shuffle=on ./...
```

### Test patterns

The codebase uses these patterns consistently:

**In-memory SQLite** -- Registry and source tests open a `:memory:` SQLite database. No test fixtures, no cleanup needed.

```go
reg, err := registry.Open(":memory:")
```

**fakePlugin** -- Provisioner tests use a hand-rolled fake that implements `DeploymentPlugin` with in-memory state. No mocking frameworks.

**httptest** -- Panel API tests use `httptest.NewServer` or `httptest.NewRecorder` for HTTP handler testing.

**Table-driven tests** -- Most test files use the `tests := []struct{ ... }` pattern with `t.Run(tt.name, ...)`.

### Conformance suites

Three conformance suites run against a real Docker environment:

```bash
# Runtime conformance (24 checks: auth, health, SSE, security)
go run ./cmd/fox-control conformance run --image <fox-image>

# Plugin conformance (8 checks: full lifecycle)
go run ./cmd/fox-control conformance plugin --image <fox-image>

# Data plane conformance (10 checks: health, auth, source CRUD, query)
go test ./conformance/dataplane/...
```

The runtime and plugin suites have dedicated CLI subcommands. The data plane suite runs via `go test`. All three require Docker and a built Fox container image. They are not part of the unit test suite.

### Benchmarks

Benchmark tests exist for hot paths:

```bash
go test -bench=. -benchmem ./internal/registry/
go test -bench=. -benchmem ./internal/events/
go test -bench=. -benchmem ./skillsets/
go test -bench=. -benchmem ./panel/api/
```

---

## 6. Build and Run

### Building

```bash
go build -o fox-control ./cmd/fox-control
```

With version info:

```bash
go build -ldflags "-X main.buildVersion=v0.1.0 -X main.buildCommit=$(git rev-parse --short HEAD) -X main.buildDate=$(date -u +%Y-%m-%d)" -o fox-control ./cmd/fox-control
```

### Configuration

Default config path: `/etc/fox-control/fox-control.toml`

Override with `--config`:

```bash
./fox-control serve --config ./fox-control.toml
```

Minimum development config:

```toml
[control]
listen = "127.0.0.1:9090"
data_root = "/tmp/fox-data"

[docker]
image = "ghcr.io/fox-in-the-box-ai/cloud:stable"

[auth]
admin_secret = "dev-secret-at-least-16chars"
instance_password = "dev-instance-password"

[instances]
port_start = 8787
max_instances = 2
```

### Environment variable overrides

| Variable | Overrides | Purpose |
|----------|-----------|---------|
| `FOX_ADMIN_SECRET` | `auth.admin_secret` | Admin API authentication secret |
| `FOX_INSTANCE_PASSWORD` | `auth.instance_password` | Password injected into Fox instances |

### Data plane config (optional)

To enable the data plane, add:

```toml
[qdrant]
enabled = true
image = "qdrant/qdrant:v1.14.1"
http_port = 6333
grpc_port = 6334

[data_plane]
enabled = true
listen = "127.0.0.1:9091"
collection = "fox-knowledge"
vector_size = 1536

[embedding]
base_url = "http://localhost:11434/v1"
model = "nomic-embed-text"
```

### Key subcommands

```bash
# Start the server (panel + optional data plane)
./fox-control serve

# Provision a new instance
./fox-control provision --id my-fox

# List all instances
./fox-control list
./fox-control list -o json
./fox-control list -o quiet

# Destroy an instance
./fox-control destroy --id my-fox --remove-data

# Rolling update all instances to a new image
./fox-control rollout --image ghcr.io/fox-in-the-box-ai/fox@sha256:abc123...

# Generate a secret suitable for admin_secret
./fox-control generate-secret

# Rotate the SSE signing key
./fox-control sec rotate-sse-key

# Rotate an instance's data plane query token
./fox-control sec rotate-query-token --instance my-fox

# Run conformance suites
./fox-control conformance run --image <fox-image>
./fox-control conformance plugin --image <fox-image>
```

### SQLite databases

All databases live under `control.data_root`:

| File | Purpose |
|------|---------|
| `registry.db` | Instance registry, signing keys |
| `events.db` | Persisted event log |
| `sources.db` | Data plane source metadata and document tracking |

All databases use WAL mode with a 5-second busy timeout. Max open connections is set to 1 (serialized writes, concurrent reads via WAL).
