# Deployment Guide

Fox Fleet runs on any Linux host with Docker. This guide covers four deployment methods — pick the one that fits your infrastructure.

| Method                            | Best for                                   | TLS                    | Data plane                |
| --------------------------------- | ------------------------------------------ | ---------------------- | ------------------------- |
| [Docker Compose](#docker-compose) | Quickstart, single host, dev/staging       | Via Caddy add-on       | Included (Qdrant sidecar) |
| [Helm chart](#helm-kubernetes)    | Kubernetes clusters                        | Via Ingress controller | Bring your own Qdrant     |
| [systemd](#systemd-bare-metal)    | Bare-metal / VM, no container orchestrator | Via Caddy add-on       | Bring your own Qdrant     |
| [Binary](#binary-manual)          | Development, testing, CI                   | None (localhost)       | Optional                  |

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
FOX_VERSION=latest          # pin to a release tag (e.g. 1.5.0)
FOX_LISTEN_PORT=9090        # host port for the panel
QDRANT_HTTP_PORT=6333       # host port for Qdrant HTTP
QDRANT_GRPC_PORT=6334       # host port for Qdrant gRPC
```

### 3. Review the config

`fox-control.toml` ships with working defaults. The auth section can be left empty in the TOML; environment variables `FOX_ADMIN_SECRET` and `FOX_INSTANCE_PASSWORD` override TOML values at runtime:

```toml
[auth]
admin_secret = ""
instance_password = ""
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

Data lives in two places: the host bind mount `/var/lib/fox-control` (SQLite registry + instance configs — a host path, not a named volume, so Fox instance containers can bind-mount the same files; see #180) and the Docker volume `qdrant-data` (vector storage). Note `docker compose down -v` only deletes `qdrant-data`; the bind-mounted data root persists on the host.

Before first start, create the data root with restricted permissions (the standard compose example runs the container as root, so root ownership is correct; for Cloud mode's non-root container, chown to uid 65532 as shown in the Cloud section):

```bash
sudo mkdir -p /var/lib/fox-control && sudo chmod 0700 /var/lib/fox-control
```

**Upgrading from a pre-1.8.0 compose file** (which used the `fox-data` named volume): your existing registry and instance configs are inside that volume, not at the host path. Find the volume name with `docker volume ls` (typically `<project>_fox-data`, e.g. `fox-fleet_fox-data`), then move the data once before switching:

```bash
docker compose down
sudo cp -a "$(docker volume inspect <project>_fox-data --format '{{.Mountpoint}}')/." /var/lib/fox-control/
docker compose up -d      # with the new bind-mount compose file
docker volume rm <project>_fox-data   # after verifying the instances list
```

---

## Cloud mode

Cloud mode serves each user's Fox instance on its own subdomain (`<username>.<domain>`) with per-user login sessions and automatic TLS certificate issuance. This section covers the full setup — from DNS to first user.

### Prerequisites

Everything in the [general prerequisites](#prerequisites), plus:

- **A domain you control** with DNS access (e.g. `fleet.example.com`)
- **Wildcard DNS** — an A record for `*.fleet.example.com` pointing to your server's public IP
- **Base domain DNS** — an A record for `fleet.example.com` pointing to the same IP
- **Ports 80 and 443** open in your firewall (Caddy needs both for ACME challenges and HTTPS)

Verify DNS:

```bash
dig +short fleet.example.com        # → your server IP
dig +short test.fleet.example.com   # → same IP (wildcard)
```

### 1. Create the install and data directories

```bash
sudo mkdir -p /opt/fox-fleet
sudo mkdir -p /var/lib/fox-control
sudo chown -R 65532:65532 /var/lib/fox-control
cd /opt/fox-fleet
```

The data directory (`/var/lib/fox-control`) must be owned by uid 65532 — the non-root user inside the fox-control container.

### 2. Write the environment file

```bash
cat > .env <<EOF
FOX_ADMIN_SECRET=$(openssl rand -hex 32)
FOX_INSTANCE_PASSWORD=$(openssl rand -hex 32)
DOMAIN=fleet.example.com
DOCKER_GID=$(stat -c '%g' /var/run/docker.sock)
EOF
```

Save the `FOX_ADMIN_SECRET` value — you need it to manage Fleet via the API.

`DOCKER_GID` is the group ID that owns `/var/run/docker.sock`. The fox-control container needs this to manage Docker containers.

### 3. Write the config file

```bash
cat > fox-control.toml <<'EOF'
[control]
listen = "127.0.0.1:9090"
data_root = "/var/lib/fox-control"
health_poll_seconds = 15
log_format = "json"
log_level = "info"

[docker]
socket = "/var/run/docker.sock"
image = "ghcr.io/fox-in-the-box-ai/cloud:latest"

[cloud]
enabled = true
domain = "fleet.example.com"
EOF
```

Replace `fleet.example.com` with your domain. The `[cloud]` section enables subdomain routing, the Cloud login page, and the user management API.

### 4. Write the Caddyfile

```bash
cat > Caddyfile <<'CADDYEOF'
{
    on_demand_tls {
        ask http://localhost:9090/cloud/tls-check
    }
}

fleet.example.com {
    handle /v1/* {
        reverse_proxy localhost:9091
    }

    handle /healthz {
        reverse_proxy localhost:9090
    }

    handle {
        reverse_proxy localhost:9090
    }
}

*.fleet.example.com {
    tls {
        on_demand
    }

    reverse_proxy localhost:9090
}
CADDYEOF
```

Replace `fleet.example.com` everywhere with your domain. The `on_demand_tls` block tells Caddy to validate certificate requests against fox-control's TLS-check endpoint — Caddy only issues certs for users that exist with a bound instance.

### 5. Write the Compose file

Cloud mode requires `network_mode: host` for both services so that fox-control and Caddy share the host network namespace (Caddy reaches fox-control on localhost, and fox-control reaches instance containers on their allocated ports).

```bash
cat > docker-compose.yml <<'COMPOSEEOF'
services:
  fox-control:
    image: ghcr.io/fox-in-the-box-ai/fox-control:${FOX_VERSION:-latest}
    network_mode: host
    group_add:
      - "${DOCKER_GID:-999}"
    volumes:
      - /var/lib/fox-control:/var/lib/fox-control
      - ./fox-control.toml:/etc/fox-control/fox-control.toml:ro
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - FOX_ADMIN_SECRET=${FOX_ADMIN_SECRET:?Set FOX_ADMIN_SECRET in .env}
      - FOX_INSTANCE_PASSWORD=${FOX_INSTANCE_PASSWORD:?Set FOX_INSTANCE_PASSWORD in .env}
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:9090/healthz"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s

  caddy:
    image: caddy:2-alpine
    network_mode: host
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
      - caddy_config:/config
    environment:
      DOMAIN: ${DOMAIN:?Set DOMAIN in .env}
    restart: unless-stopped

volumes:
  caddy_data:
  caddy_config:
COMPOSEEOF
```

### 6. Start the stack

```bash
docker compose up -d
```

Verify:

```bash
# fox-control healthy
curl -s http://localhost:9090/healthz
# → {"status":"ok"}

# Caddy serving HTTPS
curl -sI https://fleet.example.com/healthz
# → HTTP/2 200
```

### 7. Create a user and provision an instance

Cloud mode uses the user management API. Each user gets one Fox instance, served at `<username>.<domain>`.

```bash
# Read admin secret from .env
source .env

# Create a user
curl -X POST http://localhost:9090/api/users \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"username": "alice", "password": "secure-password-here"}'

# Provision an instance with matching owner
curl -X POST http://localhost:9090/api/instances \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"id": "alice", "owner": "alice"}'
```

The instance `id` must equal the `username` — this is the slug = username invariant that Cloud mode enforces. The `owner` field auto-binds the user to the instance.

Wait for the instance to become healthy (first run pulls the Fox image, ~1-2 minutes):

```bash
curl -s http://localhost:9090/api/instances/alice \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET"
# → {"id":"alice","status":"running",...}
```

### 8. Verify the subdomain

```bash
curl -sI https://alice.fleet.example.com/
# → HTTP/2 200, redirects to /login
```

Open `https://alice.fleet.example.com/` in a browser. The login page shows "Sign in as alice" with a password field only — the username is inferred from the subdomain. After login, the user sees their Fox AI assistant.

### Cloud mode API reference

| Endpoint                             | Method    | Description                                                           |
| ------------------------------------ | --------- | --------------------------------------------------------------------- |
| `POST /api/users`                    | Create    | Create a user (`{"username":"...","password":"..."}`)                 |
| `GET /api/users`                     | List      | List all users                                                        |
| `GET /api/users/{username}`          | Read      | Get user details (includes `instance_id`)                             |
| `PUT /api/users/{username}`          | Update    | Update user fields (`{"instance_id":"..."}`)                          |
| `DELETE /api/users/{username}`       | Delete    | Delete a user                                                         |
| `POST /api/instances/provision`      | Provision | Combined user + instance creation (`{"slug":"...","password":"..."}`) |
| `POST /api/instances/{id}/upgrade`   | Upgrade   | Per-instance image rollout (`{"target_image":"repo@sha256:..."}`)     |
| `GET /cloud/tls-check?domain=<fqdn>` | TLS check | Caddy's on-demand TLS validation (internal, loopback only)            |

The provision endpoint (`POST /api/instances/provision`) combines user creation and instance provisioning in one call — it creates the user, provisions the instance, and auto-binds them. The `slug` must equal the username.

### Upgrading Cloud mode

```bash
cd /opt/fox-fleet

# Update the image tag in docker-compose.yml
sed -i 's|fox-control:[0-9.]*|fox-control:NEW_VERSION|' docker-compose.yml

# Pull and restart
docker compose pull fox-control
docker compose up -d fox-control
```

Fox instances are unaffected by fox-control upgrades — they continue running independently.

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

| Value                 | Default                                  | Description                                    |
| --------------------- | ---------------------------------------- | ---------------------------------------------- |
| `image.repository`    | `ghcr.io/fox-in-the-box-ai/fox-control`  | Container image                                |
| `image.tag`           | Chart appVersion                         | Image tag                                      |
| `service.port`        | `9090`                                   | Service port                                   |
| `persistence.enabled` | `true`                                   | Persistent volume for SQLite + instance data   |
| `persistence.size`    | `1Gi`                                    | Volume size                                    |
| `config.maxInstances` | `10`                                     | Instance cap                                   |
| `config.dockerImage`  | `ghcr.io/fox-in-the-box-ai/cloud:stable` | Fox instance image                             |
| `config.portStart`    | `8787`                                   | First instance port                            |
| `auth.existingSecret` | `""`                                     | Use an existing Secret instead of creating one |
| `ingress.enabled`     | `false`                                  | Enable Ingress resource                        |

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
- Restart on any exit (`Restart=always`) with 5-attempt rate limit (`StartLimitBurst=5` in 300 seconds)

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
image = "ghcr.io/fox-in-the-box-ai/cloud:stable"

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
listen = "127.0.0.1:9090"          # Address:port for the panel API (default: 127.0.0.1:9090)
data_root = "/var/lib/fox-control" # Instance data directories (required)
health_poll_seconds = 15           # Health check interval (default: 15, range: 1–3600)
session_token_ttl_seconds = 600    # SSE session token TTL (default: 600, range: 60–3600)
log_format = "text"                # Log format: "text" or "json" (default: "text")
log_level = "info"                 # Log level: debug, info, warn, error (default: "info")
# metrics_enabled = true           # Prometheus /metrics endpoint (default: true)

[docker]
socket = "/var/run/docker.sock"    # Docker daemon socket (default: /var/run/docker.sock)
image = "ghcr.io/fox-in-the-box-ai/cloud:stable"  # Default Fox image (required)

[auth]
admin_secret = ""                  # Required — panel API bearer token (min 16 chars)
instance_password = ""             # Required — injected into instances

[instances]
port_start = 8787                  # First allocated instance port (default: 8787)
max_instances = 10                 # Instance cap (default: 2, range: 1–1000)
# default_skillset = ""           # Default skillset YAML path
# default_role = ""               # Default instance role

[tls]
# cert_file = "/path/to/cert.pem" # TLS certificate (both cert_file and key_file must be set)
# key_file = "/path/to/key.pem"   # TLS private key

[qdrant]
enabled = false                    # Enable Qdrant sidecar management
image = "qdrant/qdrant:v1.14.1"   # Qdrant container image (required when enabled)
http_port = 6333                   # Qdrant HTTP API port (required when enabled)
grpc_port = 6334                   # Qdrant gRPC API port (required when enabled)
# data_dir = ""                   # Qdrant data directory (defaults to <data_root>/qdrant)

[data_plane]
enabled = false                    # Enable knowledge ingestion + query API
listen = "127.0.0.1:9091"         # Data plane listen address (default: 127.0.0.1:9091)
collection = "fox-knowledge"      # Qdrant collection name (default: fox-knowledge)
vector_size = 1536                 # Embedding vector dimensions (default: 1536)

[embedding]
base_url = ""                      # OpenAI-compatible embedding API (required when data_plane enabled)
# api_key = ""                    # API key for embedding service (optional, for remote providers)
model = "nomic-embed-text"         # Embedding model name (required when data_plane enabled)

[rate_limit]
# requests_per_minute = 100       # General API rate limit (default: 100)
# provision_per_minute = 10       # Provision endpoint rate limit

[auto_restart]
# enabled = false                 # Auto-restart unhealthy instances
# threshold = 3                   # Consecutive failures before restart (default: 3)
# cooldown_seconds = 300          # Seconds between auto-restarts (default: 300)

# [[webhooks]]
# url = "https://ops.example.com/hooks/fox-fleet"
# secret = "whsec_your-hmac-secret"
# events = ["instance.provisioned", "instance.destroyed", "instance.unhealthy"]
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
FOX_VERSION=1.5.0 docker compose up -d

# Helm
helm upgrade fox-control deploy/helm/fox-control --set image.tag=1.5.0
```

### Binary deployments (systemd / manual)

Download the new binary, replace `/usr/local/bin/fox-control`, and restart:

```bash
sudo systemctl restart fox-control
```

### Rolling instance updates

**Via API (per-instance):**

```bash
curl -X POST https://your-fleet-host/api/instances/<id>/upgrade \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"target_image": "ghcr.io/fox-in-the-box-ai/cloud@sha256:<digest>"}'
```

The upgrade endpoint accepts a digest reference (`repo@sha256:...`) or a tag reference (`repo:tag`). Rollout is synchronous — the API blocks until the instance is healthy on the new image or rolls back on failure. The response includes `previous_digest` and `current_digest` for verification.

If the instance is already running the target digest, the endpoint returns `"status": "already_current"` without restarting.

**Via CLI (fleet-wide):**

```bash
./fox-control rollout --image ghcr.io/fox-in-the-box-ai/fox@sha256:<digest>
```

Rollout is health-gated — each instance must pass health checks before the next one updates. If an instance fails health checks, the rollout stops and the failed instance rolls back automatically.

---

## Troubleshooting

| Symptom                          | Check                                                                                                                                                                 |
| -------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Panel returns 401                | Verify `FOX_ADMIN_SECRET` matches between your client and the running config. Check env var override vs. TOML value.                                                  |
| `fox-control` refuses to start   | Missing `admin_secret` or `instance_password`. Both are required.                                                                                                     |
| Instance stuck in "provisioning" | Check Docker daemon is running. Check `docker.image` is pullable. Check port range is available.                                                                      |
| Data plane queries return 503    | Qdrant is unreachable. Verify Qdrant is running and the configured ports are correct.                                                                                 |
| SSE not working behind proxy     | Ensure your reverse proxy disables response buffering. Caddy and the `X-Accel-Buffering: no` header handle this automatically. For nginx, add `proxy_buffering off;`. |
| Panel shows stale data           | SSE may have disconnected. The panel falls back to 5-second polling. Hard refresh or check browser DevTools for EventSource errors.                                   |

---

## Architecture notes

For known limitations (single-host, no HA, single auth token, etc.), see [LIMITATIONS.md](LIMITATIONS.md).

For the full architecture, plugin interface, and ecosystem context, see the [project README](https://github.com/fox-in-the-box-ai/fox-fleet#readme).
