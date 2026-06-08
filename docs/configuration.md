# Configuration Reference

Fox Fleet is configured via a TOML file, typically at `/etc/fox-control/fox-control.toml`. Override the path with `--config`.

---

## Full example

```toml
[control]
listen = "127.0.0.1:9090"
data_root = "/var/lib/fox-control"
health_poll_seconds = 30

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

[data_plane]
enabled = false
listen = "127.0.0.1:9091"
qdrant_url = "http://127.0.0.1:6334"
embedding_url = "http://127.0.0.1:11434"
embedding_model = "nomic-embed-text"
```

---

## Sections

### `[control]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `listen` | string | `"127.0.0.1:9090"` | Address for the panel HTTP server |
| `data_root` | string | `"/var/lib/fox-control"` | Root directory for instance data, registry, and skillsets |
| `health_poll_seconds` | int | `30` | Interval between health checks for all instances |

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

### `[data_plane]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | bool | `false` | Enable the knowledge data plane |
| `listen` | string | `"127.0.0.1:9091"` | Address for the data plane HTTP server |
| `qdrant_url` | string | `"http://127.0.0.1:6334"` | Qdrant vector database REST API URL |
| `embedding_url` | string | *(required if enabled)* | OpenAI-compatible embedding API URL |
| `embedding_model` | string | *(required if enabled)* | Embedding model name |

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
