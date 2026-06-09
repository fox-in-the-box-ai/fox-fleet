# Deployment Guide

Fox Fleet runs on any Linux host with Docker. This guide covers four deployment methods — pick the one that fits your infrastructure.

| Method | Best for | TLS | Data plane |
|--------|----------|-----|------------|
| [Docker Compose](#docker-compose) | Quickstart, single host, dev/staging | Via Caddy add-on | Included (Qdrant sidecar) |
| [Helm chart](#helm-kubernetes) | Kubernetes clusters | Via Ingress controller | Bring your own Qdrant |
| [systemd](#systemd-bare-metal) | Bare-metal / VM, no container orchestrator | Via Caddy add-on | Bring your own Qdrant |
| [Binary](#binary-manual) | Development, testing, CI | None (localhost) | Optional |

All methods share the same binary and config format. The difference is how the binary starts and how secrets are injected.

---

## Prerequisites

- **Docker** — fox-control manages Fox instances as Docker containers. The Docker daemon must be running and the fox-control process (or container) must have access to `/var/run/docker.sock`.
- **Secrets** — two values are required before first start:
  - `FOX_ADMIN_SECRET` — authenticates operator requests to the panel API. Use a random string (32+ characters). This same token is injected into each managed instance as `FOX_PLANE_AUTH_SECRET`.
  - `FOX_INSTANCE_PASSWORD` — shared password injected into each managed instance for upstream session auth.
- **Ports** — default panel port is 9090. Managed Fox instances allocate ports sequentially from 8787 (configurable). Ensure the range is open in your firewall.

Generate secrets:

```bash
openssl rand -hex 32   # FOX_ADMIN_SECRET
openssl rand -hex 32   # FOX_INSTANCE_PASSWORD
```

---

## Docker Compose

The fastest path to a running Fox Fleet with the data plane (knowledge ingestion + vector search).

### 1. Copy the deployment files

```bash
cd deploy/docker-compose
cp .env.example .env
```

### 2. Set secrets

Edit `.env` and replace the placeholder values:

```bash
FOX_ADMIN_SECRET=<your-generated-secret>
FOX_INSTANCE_PASSWORD=<your-generated-password>
```

Optional overrides:

```bash
FOX_VERSION=latest          # pin to a release tag (e.g. 1.4.2)
FOX_LISTEN_PORT=9090        # host port for the panel
QDRANT_HTTP_PORT=6333       # host port for Qdrant HTTP
QDRANT_GRPC_PORT=6334       # host port for Qdrant gRPC
```

### 3. Review the config

`fox-control.toml` ships with working defaults. The auth section reads from environment variables:

```toml
[auth]
admin_secret = "${FOX_ADMIN_SECRET}"
instance_password = "${FOX_INSTANCE_PASSWORD}"
```

Adjust `[docker].image` if your Fox image is in a different registry. Adjust `[embedding].base_url` if your embedding provider is not a local Ollama on the host (the default points to `host.docker.internal:11434`).

### 4. Start the stack

```bash
docker compose up -d
```

This starts two services:

- **fox-control** — management plane on port 9090
- **qdrant** — vector database on ports 6333/6334

Both have health checks. fox-control waits for Qdrant to be healthy before starting.

### 5. Verify

```bash
# Check service health
docker compose ps

# Check the panel is responding
curl -s http://localhost:9090/healthz
# → {"status":"ok"}

# Open the panel
open http://localhost:9090
```

Log in with your `FOX_ADMIN_SECRET` as the bearer token.

### 6. Add TLS with Caddy (optional)

For production, add the Caddy reverse proxy for automatic Let's Encrypt TLS.

```bash
cd deploy/caddy
cp .env.example .env
# Edit .env — set DOMAIN=fox.example.com
```

Run Caddy alongside the Compose stack (it expects fox-control on localhost:9090):

```bash
docker compose -f docker-compose.yml up -d
```

Or add the Caddy service to your existing Compose file. See `deploy/caddy/Caddyfile` for the full routing config — it proxies the panel, data plane API (`/v1/*`), and optionally instance subdomains (`*.your-domain.com`).

### Stopping and data

```bash
docker compose down            # stop services, keep data
docker compose down -v         # stop services AND delete volumes
```

Data is stored in two Docker volumes: `fox-data` (SQLite registry + instance configs) and `qdrant-data` (vector storage).

---

## Helm (Kubernetes)

### 1. Add the chart

The chart is in `deploy/helm/fox-control/`. Install from the local path or publish to your chart repository.

```bash
helm install fox-control deploy/helm/fox-control \
  --set auth.adminSecret="$(openssl rand -hex 32)" \
  --set auth.instancePassword="$(openssl rand -hex 32)"
```

### 2. Key values

| Value | Default | Description |
|-------|---------|-------------|
| `image.repository` | `ghcr.io/fox-in-the-box-ai/fox-control` | Container image |
| `image.tag` | Chart appVersion | Image tag |
| `service.port` | `9090` | Service port |
| `persistence.enabled` | `true` | Persistent volume for SQLite + instance data |
| `persistence.size` | `1Gi` | Volume size |
| `config.maxInstances` | `10` | Instance cap |
| `config.dockerImage` | `ghcr.io/fox-in-the-box-ai/fox:latest` | Fox instance image |
| `config.portStart` | `8787` | First instance port |
| `auth.existingSecret` | `""` | Use an existing Secret instead of creating one |
| `ingress.enabled` | `false` | Enable Ingress resource |

### 3. Docker socket access

fox-control needs the Docker socket to manage containers. On Kubernetes, this means the pod needs a `hostPath` volume mount for `/var/run/docker.sock`. The chart includes this by default.

This has security implications — the pod effectively has root access to the node's Docker daemon. Use `nodeSelector` or `affinity` to pin fox-control to a dedicated node, and enforce pod security standards on other workloads.

### 4. External Qdrant

The chart does **not** deploy Qdrant. If you need the data plane (knowledge ingestion + vector search), deploy Qdrant separately and configure:

```bash
helm install fox-control deploy/helm/fox-control \
  --set auth.adminSecret="..." \
  --set auth.instancePassword="..." \
  --set qdrant.enabled=true \
  --set dataPlane.enabled=true \
  --set embedding.baseURL="http://ollama.default.svc:11434"
```

### 5. Verify

```bash
kubectl get pods -l app.kubernetes.io/name=fox-control
kubectl port-forward svc/fox-control 9090:9090
curl http://localhost:9090/healthz
```

### 6. Ingress

Enable with:

```yaml
ingress:
  enabled: true
  className: nginx
  hosts:
    - host: fox.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: fox-tls
      hosts:
        - fox.example.com
```

---

## systemd (bare metal)

For Linux servers without a container orchestrator. Fox-control runs as a systemd service under a dedicated system user with security hardening.

### 1. Build the binary

```bash
git clone https://github.com/fox-in-the-box-ai/fox-fleet.git
cd fox-fleet
make build
```

Or download a prebuilt binary from the [GitHub releases](https://github.com/fox-in-the-box-ai/fox-fleet/releases).

### 2. Run the installer

```bash
sudo ./deploy/systemd/install.sh ./fox-control
```

The installer:
- Creates a `fox-control` system user (no login shell)
- Adds it to the `docker` group
- Installs the binary to `/usr/local/bin/fox-control`
- Creates `/etc/fox-control/` with a default config and env file
- Creates `/var/lib/fox-control/` for data
- Installs the systemd unit

### 3. Set secrets

```bash
sudo editor /etc/fox-control/env
```

```
FOX_ADMIN_SECRET=<your-generated-secret>
FOX_INSTANCE_PASSWORD=<your-generated-password>
```

The env file is mode 0600, readable only by the fox-control user.

### 4. Review config

```bash
sudo editor /etc/fox-control/fox-control.toml
```

Adjust `[docker].image`, `[instances].max_instances`, and optionally enable the data plane sections. The auth section reads from environment variables injected by the systemd `EnvironmentFile`.

### 5. Start

```bash
sudo systemctl enable --now fox-control
sudo journalctl -u fox-control -f
```

### 6. Security hardening

The systemd unit applies:

- `NoNewPrivileges=true`
- `ProtectSystem=strict` (read-only filesystem except allowed paths)
- `PrivateTmp=true`, `PrivateDevices=true`
- `RestrictNamespaces=true`, `RestrictSUIDSGID=true`
- `ReadWritePaths=/var/lib/fox-control` (only writable path)
- Capability bounding set is empty (no capabilities)
- Restart on failure with 5-attempt rate limit

The fox-control user's only privilege escalation path is the Docker socket (via group membership). This is inherent to the architecture — see [LIMITATIONS.md](LIMITATIONS.md).

---

## Binary (manual)

For development, testing, or environments where you manage process supervision yourself.

### 1. Build

```bash
make build
# produces ./fox-control
```

### 2. Configure

Create a minimal `fox-control.toml`:

```toml
[control]
listen = "127.0.0.1:9090"
data_root = "./data"

[docker]
image = "ghcr.io/fox-in-the-box-ai/fox:latest"

[auth]
admin_secret = "dev-secret-change-in-prod"
instance_password = "dev-password-change-in-prod"

[instances]
max_instances = 2
```

### 3. Run

```bash
./fox-control serve --config fox-control.toml
```

Or with environment variable overrides (take precedence over TOML values):

```bash
export FOX_ADMIN_SECRET="my-secret"
export FOX_INSTANCE_PASSWORD="my-password"
./fox-control serve --config fox-control.toml
```

---

## Configuration reference

All deployment methods use the same TOML config format.

```toml
[control]
listen = "0.0.0.0:9090"       # Address:port for the panel API
data_root = "/var/lib/fox-control"  # Instance data directories
health_poll_seconds = 15       # Health check interval

[docker]
socket = "/var/run/docker.sock"  # Docker daemon socket
image = "ghcr.io/fox-in-the-box-ai/fox:latest"  # Default Fox image

[auth]
admin_secret = ""              # Required — panel API bearer token
instance_password = ""         # Required — injected into instances

[instances]
port_start = 8787              # First allocated instance port
max_instances = 10             # Instance cap
# default_skillset = ""        # Default skillset YAML path
# default_role = ""            # Default instance role

[qdrant]
enabled = false                # Enable Qdrant sidecar management
http_port = 6333
grpc_port = 6334

[data_plane]
enabled = false                # Enable knowledge ingestion + query API
listen = "0.0.0.0:9091"
collection = "fox-knowledge"
vector_size = 1536

[embedding]
base_url = ""                  # OpenAI-compatible embedding API (e.g. http://localhost:11434)
model = "nomic-embed-text"
```

Environment variables `FOX_ADMIN_SECRET` and `FOX_INSTANCE_PASSWORD` override the TOML `[auth]` values when set. This is the recommended approach for production — keep secrets out of config files.

---

## Post-deployment verification

After starting fox-control by any method:

### Health check

```bash
curl http://localhost:9090/healthz
# {"status":"ok"}
```

### Panel access

Open `http://localhost:9090` (or your TLS domain) in a browser. Enter your admin secret to log in. The panel shows instance status, sources, skillsets, and an activity feed.

### Provision a test instance

Via the panel "Create Instance" button, or via API:

```bash
curl -X POST http://localhost:9090/api/instances \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"id": "test-fox"}'
```

### Real-time updates

The panel connects to `GET /api/events/stream` via Server-Sent Events. Lifecycle events (provision, destroy, health changes) appear in the activity feed within seconds. If SSE is unavailable (proxy buffering, network issues), the panel falls back to 5-second polling automatically.

---

## Upgrading

### Container deployments (Compose / Helm)

```bash
# Docker Compose
FOX_VERSION=1.4.2 docker compose up -d

# Helm
helm upgrade fox-control deploy/helm/fox-control --set image.tag=1.4.2
```

### Binary deployments (systemd / manual)

Download the new binary, replace `/usr/local/bin/fox-control`, and restart:

```bash
sudo systemctl restart fox-control
```

### Rolling instance updates

To update the Fox instance image across all managed instances:

```bash
./fox-control rollout --image ghcr.io/fox-in-the-box-ai/fox@sha256:<digest>
```

Rollout is health-gated — each instance must pass health checks before the next one updates. If an instance fails health checks, the rollout stops and the failed instance rolls back automatically.

---

## Troubleshooting

| Symptom | Check |
|---------|-------|
| Panel returns 401 | Verify `FOX_ADMIN_SECRET` matches between your client and the running config. Check env var override vs. TOML value. |
| `fox-control` refuses to start | Missing `admin_secret` or `instance_password`. Both are required. |
| Instance stuck in "provisioning" | Check Docker daemon is running. Check `docker.image` is pullable. Check port range is available. |
| Data plane queries return 503 | Qdrant is unreachable. Verify Qdrant is running and the configured ports are correct. |
| SSE not working behind proxy | Ensure your reverse proxy disables response buffering. Caddy and the `X-Accel-Buffering: no` header handle this automatically. For nginx, add `proxy_buffering off;`. |
| Panel shows stale data | SSE may have disconnected. The panel falls back to 5-second polling. Hard refresh or check browser DevTools for EventSource errors. |

---

## Architecture notes

For known limitations (single-host, no HA, single auth token, etc.), see [LIMITATIONS.md](LIMITATIONS.md).

For the full architecture, plugin interface, and ecosystem context, see the [project README](https://github.com/fox-in-the-box-ai/fox-fleet#readme).
