# Configuration Reference

Fox Fleet is configured via a TOML file, typically at `/etc/fox-control/fox-control.toml`. Override the path with `--config`.

---

## Full example

```toml
[control]
listen = "127.0.0.1:9090"
data_root = "/var/lib/fox-control"
health_poll_seconds = 15
session_token_ttl_seconds = 600
log_format = "text"
log_level = "info"
metrics_enabled = true

[docker]
image = "ghcr.io/fox-in-the-box-ai/cloud:stable"
socket = "/var/run/docker.sock"

[auth]
admin_secret = "change-me-to-a-real-secret"
instance_password = "change-me-to-a-real-password"

[instances]
max_instances = 10
port_start = 8787
# default_skillset = "/path/to/skillset.yaml"
# default_role = "assistant"

[qdrant]
enabled = false
image = "qdrant/qdrant:v1.14.1"
http_port = 6333
grpc_port = 6334
# data_dir = "/var/lib/fox-control/qdrant"

[data_plane]
enabled = false
listen = "127.0.0.1:9091"
collection = "fox-knowledge"
vector_size = 1536

[embedding]
base_url = "http://127.0.0.1:11434"
# api_key = ""
model = "nomic-embed-text"

[tls]
# Both fields required to enable TLS; omit both to disable.
# cert_file = "/etc/fox-control/tls/tls.crt"
# key_file  = "/etc/fox-control/tls/tls.key"

[[webhooks]]
# url    = "https://hooks.example.com/fox-events"
# events = ["instance.created", "instance.deleted", "instance.unhealthy"]
# secret = "webhook-hmac-secret"

[cloud]
# enabled = false
# domain  = "fleet.example.com"

[rate_limit]
# requests_per_minute  = 100  # default: 100
# provision_per_minute = 0

[auto_restart]
# enabled          = false
# threshold        = 3      # consecutive failures before restart
# cooldown_seconds = 300    # seconds between restart attempts
```

---

## Sections

### `[control]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `listen` | string | `"127.0.0.1:9090"` | Address for the panel HTTP server |
| `data_root` | string | *(required)* | Root directory for instance data, registry, and skillsets |
| `health_poll_seconds` | int | `15` | Interval between health checks for all instances (range: 1–3600) |
| `session_token_ttl_seconds` | int | `600` | Session token lifetime in seconds (range: 60–3600) |
| `log_format` | string | `"text"` | Log output format: `"text"` or `"json"` |
| `log_level` | string | `"info"` | Minimum log level: `debug`, `info`, `warn`, `error` |
| `metrics_enabled` | bool | `true` | Expose Prometheus-compatible `/metrics` endpoint |

### `[docker]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `image` | string | *(required)* | Fox container image reference (tag or digest) |
| `socket` | string | `"/var/run/docker.sock"` | Docker daemon socket path |

### `[auth]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `admin_secret` | string | *(required)* | Shared secret for panel authentication. Must not be empty. |
| `instance_password` | string | *(required)* | Password injected into each instance for upstream auth. Must not be empty. |

!!! warning "Environment variable overrides"
    `FOX_ADMIN_SECRET` and `FOX_INSTANCE_PASSWORD` environment variables take precedence over values in the TOML config. Use these for secrets in containerized deployments.

### `[instances]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `max_instances` | int | `2` | Maximum number of concurrent instances |
| `port_start` | int | `8787` | First port in the allocation range |
| `default_skillset` | string | `""` | Path to a default skillset manifest assigned to new instances |
| `default_role` | string | `""` | Default principal role for new instances |

### `[qdrant]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | bool | `false` | Enable the managed Qdrant sidecar container |
| `image` | string | `""` | Qdrant container image reference |
| `http_port` | int | `0` | Qdrant HTTP API port |
| `grpc_port` | int | `0` | Qdrant gRPC API port |
| `data_dir` | string | `"<data_root>/qdrant"` | Qdrant data directory. Defaults to `<data_root>/qdrant` when qdrant is enabled and `data_root` is set. *(required when qdrant is enabled)* |

### `[data_plane]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | bool | `false` | Enable the knowledge data plane |
| `listen` | string | `"127.0.0.1:9091"` | Address for the data plane HTTP server |
| `collection` | string | `"fox-knowledge"` | Qdrant collection name for knowledge vectors |
| `vector_size` | int | `1536` | Embedding vector dimensionality |

!!! note "Dependency"
    Enabling `data_plane` requires `qdrant.enabled = true` and a configured `[embedding]` section.

### `[embedding]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `base_url` | string | *(required if data_plane enabled)* | OpenAI-compatible embedding API base URL |
| `api_key` | string | `""` | API key for the embedding service. Leave empty for local models (e.g. Ollama). |
| `model` | string | *(required if data_plane enabled)* | Embedding model name |

!!! warning "Secret handling"
    `embedding.api_key` is a secret. Use environment-based injection or a secrets manager in production deployments rather than storing it in the TOML file.

### `[tls]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `cert_file` | string | `""` | Path to TLS certificate file. Both `cert_file` and `key_file` must be set or both empty. |
| `key_file` | string | `""` | Path to TLS private key file |

### `[[webhooks]]`

Repeatable TOML array of tables. Each entry defines a webhook receiver.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `url` | string | *(required)* | Webhook receiver URL |
| `events` | string[] | `[]` | Event types to deliver (e.g. `instance.created`, `instance.deleted`, `instance.unhealthy`) |
| `secret` | string | `""` | HMAC signing secret for webhook payloads |

### `[cloud]`

Enables Cloud mode — multi-user subdomain routing with per-instance sessions and on-demand TLS.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | bool | `false` | Enable Cloud mode (subdomain-per-user routing, Cloud login page, user management API) |
| `domain` | string | *(required when enabled)* | Base domain for subdomain routing. Each user's Fox instance is served at `<username>.<domain>`. Wildcard DNS must point `*.<domain>` to the Fleet host. |

!!! note "Cloud mode requirements"
    When `cloud.enabled = true`:
    - `domain` is required — fox-control refuses to start without it.
    - A reverse proxy (Caddy recommended) must handle TLS termination for `<domain>` and `*.<domain>`.
    - Users are managed via the `/api/users` endpoints (create, list, update, delete).
    - Each user is bound to one instance via `instance_id`. The instance `id` must equal the `username` (slug = username invariant).
    - The panel serves a Cloud login page at `/cloud/login` instead of the default admin panel.
    - See [Cloud mode deployment](DEPLOYMENT.md#cloud-mode) for the full setup guide.

### `[rate_limit]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `requests_per_minute` | int | `100` | Global API request rate limit (defaults to 100 when unset or 0) |
| `provision_per_minute` | int | `0` | Provision endpoint rate limit |

### `[auto_restart]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | bool | `false` | Automatically restart unhealthy instances |
| `threshold` | int | `3` | Consecutive health-check failures before triggering a restart |
| `cooldown_seconds` | int | `300` | Minimum seconds between automatic restarts of the same instance |

---

## CLI flags

Global flags available to all subcommands:

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `/etc/fox-control/fox-control.toml` | Path to config file |
| `-o`, `--output` | `table` | Output format: `table`, `json`, `quiet` |

### Subcommands

| Command | Description |
|---------|-------------|
| `fox-control serve` | Start the panel HTTP server |
| `fox-control provision --id <id>` | Provision a new instance |
| `fox-control destroy --id <id> [--remove-data]` | Destroy an instance |
| `fox-control list` | List all instances |
| `fox-control rollout --image <ref>` | Rolling update to a new image (requires digest reference) |
| `fox-control version` | Print version information |
| `fox-control verify <file>` | Verify cosign signature of a release artifact |
| `fox-control backup --output <dir>` | Back up all databases (VACUUM INTO) to a directory |
| `fox-control restore --input <dir>` | Restore databases from a backup directory |
| `fox-control diagnostics` | Print diagnostic information for troubleshooting |
| `fox-control generate-secret [--bytes N]` | Generate a cryptographically random hex secret (default 32 bytes) |
| `fox-control sec rotate-sse-key` | Rotate the SSE session token signing key |
| `fox-control sec rotate-query-token --instance <id>` | Rotate the data plane query token for an instance |
| `fox-control conformance run --image <img>` | Run runtime conformance suite |
| `fox-control conformance plugin --image <img>` | Run plugin conformance suite |

---

## Instance data directory layout

When Fleet provisions a Fox instance, it creates a data directory under `<data_root>/instances/<instance-id>/` and writes configuration files before starting the container. The directory is bind-mounted into the Fox container as `/data`.

### Files written by Fleet

| Host path | Container path | Permissions | Purpose |
|-----------|---------------|-------------|---------|
| `<data_dir>/config/hermes.env` | `/data/config/hermes.env` | `0600` | Environment variables for the Fox runtime: auth secrets, proxy config, Cloud-mode CSRF/CSP origins, data plane token, custom operator env. Sourced by `run-with-env.sh` on each supervisord restart (hot-reload path). |
| `<data_dir>/config.yaml` | `/data/config.yaml` | `0644` | Hermes configuration overlay (model defaults, title). |
| `<data_dir>/settings.json` | `/data/settings.json` | `0644` | Hermes WebUI settings (model filter list). |
| `<data_dir>/tools.json` | `/data/tools.json` | `0600` | Tool/function definitions injected into the runtime. |

Fleet also calls `MarkOnboardingComplete()`, which writes:

| Host path | Container path | Permissions | Purpose |
|-----------|---------------|-------------|---------|
| `<data_dir>/config/onboarding_complete` | `/data/config/onboarding_complete` | `0644` | Marker file that skips the Fox onboarding wizard. |

### Cloud-mode environment variables

When `[cloud]` is enabled and `domain` is set, Fleet injects these into both `hermes.env` (for hot-reload) and the Docker container's initial environment (for first-boot):

| Variable | Value | Purpose |
|----------|-------|---------|
| `HERMES_WEBUI_ALLOWED_ORIGINS` | `https://<slug>.<domain>` | CSRF origin allowlist |
| `HERMES_WEBUI_TRUST_FORWARDED_HOST` | `true` | Trust `X-Forwarded-Host` behind the Fleet reverse proxy |
| `HERMES_WEBUI_CSP_CONNECT_EXTRA` | `https://<slug>.<domain> wss://<slug>.<domain>` | Content Security Policy connect-src additions |

These keys are blocked from user-supplied custom env (`[instances]` env overrides) to prevent accidental override of system-managed values.

### Rollback behavior

If provisioning fails at any stage after the data directory is created, Fleet removes the entire `<data_dir>` tree (including all config files and the `config/` subdirectory) and deletes the registry entry. No secret material persists after a failed provision.
