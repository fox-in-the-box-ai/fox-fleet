# Configuration Reference

Fox Fleet is configured via a TOML file, typically at `/etc/fox-control/fox-control.toml`. Override the path with `--config`.

---

## Full example

```toml
[control]
listen = "127.0.0.1:9090"
data_root = "/var/lib/fox-control"
health_poll_seconds = 15

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
image = "qdrant/qdrant:v1.13.3"
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
```

---

## Sections

### `[control]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `listen` | string | `"127.0.0.1:9090"` | Address for the panel HTTP server |
| `data_root` | string | *(required)* | Root directory for instance data, registry, and skillsets |
| `health_poll_seconds` | int | `15` | Interval between health checks for all instances |

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

---

## CLI flags

Global flags available to all subcommands:

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `/etc/fox-control/fox-control.toml` | Path to config file |

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
| `fox-control conformance run --image <img>` | Run runtime conformance suite |
| `fox-control conformance plugin --image <img>` | Run plugin conformance suite |
