# Operator Handbook

Day-2 operations for Fox Fleet. This guide assumes you have a running
deployment — see the [Quickstart](../quickstart/linux.md) or
[Deployment Guide](../DEPLOYMENT.md) if you're starting from zero.

---

## Production deployment checklist

Before serving real users, verify each item:

- [ ] **Secrets are random and unique** — `FOX_ADMIN_SECRET` and
  `FOX_INSTANCE_PASSWORD` are cryptographically random (32+ hex
  characters). Not the demo values from the walkthrough.
- [ ] **Secrets are not in config files** — use environment variables
  (`FOX_ADMIN_SECRET`, `FOX_INSTANCE_PASSWORD`) or a secrets manager.
  The TOML `[auth]` section is a fallback; env vars take precedence.
- [ ] **TLS is enabled** — either configure built-in TLS via
  `tls.cert_file` and `tls.key_file` in `fox-control.toml`, or
  terminate TLS at a reverse proxy (Caddy, nginx, cloud load
  balancer). See [Caddy add-on](../DEPLOYMENT.md#6-add-tls-with-caddy-optional).
  Do not serve plain HTTP to the internet.
- [ ] **Listen address is restricted** — in production, bind to
  `127.0.0.1:9090` (behind a reverse proxy) or a private interface.
  Never bind `0.0.0.0` without a firewall or reverse proxy.
- [ ] **Firewall rules cover the instance port range** — instances
  allocate ports from `instances.port_start` (default 8787) upward.
  Open only the ports you need; block external access to instance
  ports if the instances are accessed through a proxy.
- [ ] **Docker socket is protected** — fox-control needs
  `/var/run/docker.sock`. This is root-equivalent access. Run
  fox-control under a dedicated user with only `docker` group
  membership. Do not expose the socket over TCP.
- [ ] **Data directory has restricted permissions** — `/var/lib/fox-control`
  should be owned by the fox-control user, mode 0700. Contains the
  SQLite registry and instance configs (which include secrets).
- [ ] **Backups are configured** — see [Backup and recovery](#backup-and-recovery).
- [ ] **Monitoring is in place** — see [Monitoring](#monitoring).
- [ ] **Instance cap is set** — `instances.max_instances` defaults to 2.
  Set it to match your host capacity.

---

## Instance lifecycle

### Provision an instance

**Panel:** Click **Create Instance**, enter an instance ID, optionally
select a skillset and role.

**API:**

```bash
curl -X POST http://localhost:9090/api/instances \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"id": "team-fox-1"}'
```

**CLI:**

```bash
fox-control provision --id team-fox-1 --config /etc/fox-control/fox-control.toml
```

The provisioner allocates the next available port, writes instance
config files, pulls the Fox image (if not cached), and starts the
container. The instance transitions through: provisioning → starting →
healthy. Health checks run at the interval set by
`control.health_poll_seconds` (default 15 seconds).

### List instances

**API:**

```bash
curl -s http://localhost:9090/api/instances \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET"
```

**CLI:**

```bash
fox-control list --config /etc/fox-control/fox-control.toml
```

### Get instance details

**API:**

```bash
curl -s http://localhost:9090/api/instances/team-fox-1 \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET"
```

Returns: instance ID, status, port, image, health state, timestamps,
assigned skillset, and role.

### Destroy an instance

**Panel:** Click the instance → **Destroy** → confirm.

**API:**

```bash
curl -X DELETE http://localhost:9090/api/instances/team-fox-1 \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET"
```

**CLI:**

```bash
fox-control destroy --id team-fox-1 --config /etc/fox-control/fox-control.toml
# Add --remove-data to also delete the instance's data directory
```

Destroying stops and removes the container. Instance data (config,
conversation history) is preserved under `data_root/<instance-id>/`
unless `--remove-data` is specified.

---

## Secret rotation

### Rotate the admin secret

1. Generate a new secret:

   ```bash
   openssl rand -hex 32
   ```

2. Update the secret in your deployment:

   - **Docker Compose:** edit `.env`, set `FOX_ADMIN_SECRET=<new-value>`,
     then `docker compose up -d` (fox-control restarts with the new
     value).
   - **systemd:** edit `/etc/fox-control/env`, then
     `sudo systemctl restart fox-control`.
   - **Helm:** `helm upgrade fox-control deploy/helm/fox-control --set auth.adminSecret=<new-value>`

3. Update your browser bookmark / saved token — the old secret is
   immediately invalid.

All active panel sessions are invalidated on rotation. Users must
log in again with the new secret.

### Rotate the instance password

Same process as the admin secret, but update `FOX_INSTANCE_PASSWORD`.
After restarting fox-control, existing instances continue running with
the old password until they are re-provisioned. New instances use the
new password.

To rotate for an existing instance: destroy and re-provision it.
Instance data is preserved if `--remove-data` is not used.

---

## Skillset management

Skillsets define the tools and capabilities available to Fox instances.

### Upload a skillset

**Panel:** Skillsets tab → upload.

**API:**

```bash
curl -X POST http://localhost:9090/api/skillsets \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET" \
  -F "file=@/path/to/skillset.yaml"
```

### List skillsets

```bash
curl -s http://localhost:9090/api/skillsets \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET"
```

### Download a skillset

```bash
curl -s http://localhost:9090/api/skillsets/my-skillset/download \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET" -o skillset.yaml
```

### Delete a skillset

```bash
curl -X DELETE http://localhost:9090/api/skillsets/my-skillset \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET"
```

### Assign a default skillset

Set `instances.default_skillset` in `fox-control.toml` to the path
of a skillset manifest. All new instances are provisioned with this
skillset unless overridden at creation time.

---

## Knowledge data plane

The data plane provides knowledge ingestion and vector search. It
requires Qdrant and an embedding provider (Ollama, OpenAI-compatible
API, etc.).

### Enable the data plane

In `fox-control.toml`:

```toml
[qdrant]
enabled = true
image = "qdrant/qdrant:v1.13.3"
http_port = 6333
grpc_port = 6334

[data_plane]
enabled = true
listen = "127.0.0.1:9091"
collection = "fox-knowledge"
vector_size = 1536

[embedding]
base_url = "http://localhost:11434"
model = "nomic-embed-text"
```

Restart fox-control after changing the config.

### Create and ingest a source

Source management uses the data plane admin API (port 9091 by
default), not the panel API. Two steps: create the source definition,
then trigger ingestion.

**Step 1 — Create the source:**

```bash
curl -X POST http://localhost:9091/v1/admin/sources \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "company-handbook",
    "type": "file",
    "name": "Company Handbook",
    "config": {"path": "/path/to/document.txt"}
  }'
```

Source types: `file` (local filesystem) or `rest` (JSON endpoint).

**Step 2 — Trigger ingestion:**

```bash
curl -X POST http://localhost:9091/v1/admin/sources/company-handbook/ingest \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET"
```

The data plane chunks the document, generates embeddings via the
configured provider, and stores vectors in Qdrant.

### Query the knowledge base

**Panel:** Sources tab → Query (proxies to the data plane).

**API (via panel proxy):**

```bash
curl -X POST http://localhost:9090/api/query \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"query": "What is our vacation policy?", "top_k": 5}'
```

**API (direct to data plane):**

```bash
curl -X POST http://localhost:9091/v1/query \
  -H "Content-Type: application/json" \
  -d '{"query": "What is our vacation policy?", "top_k": 5}'
```

### List sources

**Via panel (read-only):**

```bash
curl -s http://localhost:9090/api/sources \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET"
```

**Via data plane admin API:**

```bash
curl -s http://localhost:9091/v1/admin/sources \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET"
```

### Limitations

- 50 MB per file
- Local filesystem files only (no S3, no URLs)
- Full re-ingest on change (no incremental updates)
- Single Qdrant collection shared across all instances

---

## Rolling updates

Update the Fox instance image across all managed instances with
health-gated rollback:

```bash
fox-control rollout --image ghcr.io/fox-in-the-box-ai/fox@sha256:<digest> \
  --config /etc/fox-control/fox-control.toml
```

The rollout updates one instance at a time. Each instance must pass
health checks before the next one updates. If an instance fails health
checks after the update, it rolls back automatically and the rollout
stops.

Use a digest reference (not a tag) for deterministic rollouts.

---

## Upgrading fox-control

### Docker Compose

```bash
cd deploy/docker-compose
# Edit .env: set FOX_VERSION=1.0.1 (or the target version)
docker compose pull
docker compose up -d
```

### Helm

```bash
helm upgrade fox-control deploy/helm/fox-control \
  --set image.tag=1.0.1
```

### systemd

```bash
# Download or build the new binary
sudo cp fox-control /usr/local/bin/fox-control
sudo systemctl restart fox-control
```

### Verify after upgrade

```bash
fox-control version
curl -s http://localhost:9090/healthz
```

Check the activity feed in the panel — if a schema migration ran, it
appears as an event.

---

## Backup and recovery

### What to back up

| Component | Path / volume | Contains |
|-----------|---------------|----------|
| Instance registry | `<data_root>/registry.db` | Instance metadata, ports, status, skillset assignments |
| Source registry | `<data_root>/sources.db` | Data plane source metadata |
| Event store | `<data_root>/events.db` | Persistent event history |
| Instance data | `<data_root>/<instance-id>/` | Per-instance config, conversation history, settings |
| Qdrant vectors | Qdrant data directory or Docker volume `qdrant-data` | Knowledge embeddings |
| Config | `/etc/fox-control/fox-control.toml` or `deploy/docker-compose/fox-control.toml` | Server configuration |
| Secrets | `/etc/fox-control/env` or `deploy/docker-compose/.env` | Admin secret, instance password |
| Skillsets | `<data_root>/skillsets/` | Uploaded skillset manifests |

Default `data_root`:
- Docker Compose: Docker volume `fox-data` (mounted at `/data` in the container)
- systemd: `/var/lib/fox-control`
- Binary: whatever you set in `fox-control.toml`

### Backup procedure

**CLI (recommended):**

```bash
fox-control backup --output /backup/fox-control-$(date +%Y%m%d) \
  --config /etc/fox-control/fox-control.toml
```

The `backup` command uses SQLite `VACUUM INTO` to produce consistent
snapshots of `registry.db`, `sources.db`, and `events.db` without
stopping the service. No downtime required.

To restore from a CLI backup:

```bash
# Stop fox-control first
sudo systemctl stop fox-control

fox-control restore --input /backup/fox-control-20260607 \
  --config /etc/fox-control/fox-control.toml

sudo systemctl start fox-control
```

The restore command checks for a lock file to prevent concurrent
restores. It copies the backed-up databases to the configured
`data_root`.

**Docker Compose:**

```bash
# Stop services to ensure consistency
docker compose stop

# Back up volumes
docker run --rm \
  -v fox-data:/data \
  -v $(pwd)/backups:/backup \
  alpine tar czf /backup/fox-data-$(date +%Y%m%d).tar.gz -C /data .

docker run --rm \
  -v qdrant-data:/data \
  -v $(pwd)/backups:/backup \
  alpine tar czf /backup/qdrant-data-$(date +%Y%m%d).tar.gz -C /data .

# Restart
docker compose start
```

**systemd / binary:**

```bash
sudo systemctl stop fox-control

sudo tar czf /backup/fox-control-$(date +%Y%m%d).tar.gz \
  /var/lib/fox-control \
  /etc/fox-control

sudo systemctl start fox-control
```

For minimal downtime, copy the SQLite databases while fox-control is
running (SQLite supports concurrent readers), but stop the service for
a fully consistent snapshot.

### Recovery

1. Stop fox-control.
2. Restore the backup to the original paths.
3. Start fox-control. It reads the registry and reconciles container
   state — re-creating any containers that should be running but
   aren't.

If recovering to a different host, ensure Docker is running and the
Fox instance image is pullable from the new host.

### Backup frequency

- **Registry databases:** daily, or after any batch of instance changes.
- **Qdrant:** after source ingestion. Re-ingesting from source files
  is an alternative to restoring vectors.
- **Config and secrets:** after any change. Version-control your config
  (not your secrets).

---

## Monitoring

### Health endpoint

```bash
curl -s http://localhost:9090/healthz
# {"status":"ok"}
```

Monitor this endpoint from your uptime checker. Any response other
than `{"status":"ok"}` or a connection failure means fox-control is
down.

### Instance health

fox-control polls each instance at the interval set by
`control.health_poll_seconds` (default 15). Instance health is
visible in:

- The panel dashboard (green/red indicators)
- The API: `GET /api/instances` returns `status` per instance
- The activity feed: health transitions appear as events

### Activity feed (SSE)

The panel receives real-time events via Server-Sent Events at
`GET /api/events/stream`. Lifecycle events (provision, destroy,
health changes) appear within seconds.

Events are persisted to a SQLite-backed event store (`events.db`).
The in-memory buffer still holds the 200 most recent events for fast
SSE delivery, but the full history survives restarts.

### Log output

fox-control logs to stdout. The log format is configurable via
`control.log_format`:

- `text` (default) — human-readable, one line per event.
- `json` — structured JSON, one object per line. Recommended for
  production log aggregation (ELK, Loki, Datadog, etc.).

Log verbosity is controlled by `control.log_level`: `debug`, `info`
(default), `warn`, or `error`.

In a Docker Compose deployment, use
`docker compose logs -f fox-control`. In a systemd deployment, use
`journalctl -u fox-control -f`.

Key log lines to watch for:

- `instance provisioned` — new instance started successfully
- `instance destroyed` — instance removed
- `health check failed` — an instance is unhealthy
- `rollout` — rolling update progress
- `listening on` — fox-control started and is accepting connections

### Prometheus metrics

fox-control exposes a Prometheus-compatible metrics endpoint at
`GET /metrics` when enabled. To enable, set `metrics.enabled = true`
in `fox-control.toml`.

Exposed metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `foxcontrol_requests_total` | counter | Total API requests |
| `foxcontrol_errors_total` | counter | Total error responses |
| `foxcontrol_provisions_total` | counter | Instance provision count |
| `foxcontrol_sse_connections` | gauge | Active SSE connections |
| `foxcontrol_uptime_seconds` | gauge | Process uptime in seconds |

The `/metrics` endpoint does not require authentication, so restrict
access via firewall or reverse proxy rules if needed.

Scrape config for Prometheus:

```yaml
scrape_configs:
  - job_name: fox-control
    static_configs:
      - targets: ['localhost:9090']
    metrics_path: /metrics
```

### Health history and resource statistics

The panel displays per-instance health history (24-hour timeline)
and resource usage gauges (CPU, memory, network). The underlying
data is available via the API:

- `GET /api/instances/{id}/health-history` — health event timeline
  for the last 24 hours.
- `GET /api/instances/{id}/stats` — live container resource
  statistics (CPU percentage, memory usage, network I/O).

Both endpoints require `Authorization: Bearer <admin_secret>`.

### Instance auto-restart

fox-control can automatically restart instances that fail consecutive
health checks. Enable and tune in `fox-control.toml`:

```toml
[auto_restart]
enabled = true
threshold = 3        # consecutive failures before restart
cooldown = "60s"     # minimum interval between auto-restarts
```

When enabled, the health poller tracks consecutive failures per
instance. Once the threshold is exceeded, fox-control restarts the
container and resets the counter. A cooldown prevents restart loops
for persistently failing instances. Auto-restart events appear in
the activity feed.

### Webhook event forwarding

fox-control can forward lifecycle events to external HTTP endpoints.
Configure one or more webhook targets in `fox-control.toml`:

```toml
[[webhooks]]
url = "https://ops.example.com/hooks/fox-fleet"
secret = "whsec_your-hmac-secret"
events = ["instance.provisioned", "instance.destroyed", "instance.unhealthy"]
rate_limit = 10  # max deliveries per second per target
```

Each webhook delivery includes an `X-Fox-Signature` header containing
an HMAC-SHA256 signature of the request body, computed with the
configured `secret`. Verify this signature on the receiving end to
authenticate the payload.

Event type filtering is per-target — omit the `events` key to
receive all event types. Per-target rate limiting prevents
overwhelming downstream services.

### Rate limiting

API requests are subject to token-bucket rate limiting. When a client
exceeds the allowed rate, the server returns `429 Too Many Requests`
with a `Retry-After` header indicating how many seconds to wait.

Rate limits are configurable in `fox-control.toml`:

```toml
[rate_limit]
requests_per_second = 20
burst = 50
```

The `/healthz` and `/metrics` endpoints are exempt from rate limiting.

### External monitoring integration

For production monitoring:

- Point your uptime checker at `/healthz`
- Scrape `/metrics` with Prometheus (when `metrics.enabled = true`)
- Enable structured JSON logging (`control.log_format = "json"`) for
  log aggregation
- Configure webhooks for real-time alerting on health transitions
- Monitor Docker container status for managed instances:
  `docker ps --filter label=managed-by=fox-control`

---

## Capacity guidance

### CPU and memory

fox-control itself is lightweight (single Go binary, ~20 MB RSS).
The resource bottleneck is the managed Fox instances — each runs as
a separate Docker container.

Capacity depends on the Fox image and workload. As a baseline for
the default Fox image:

- **Per instance:** ~256 MB RAM idle, more under load
- **fox-control overhead:** ~50 MB RAM + negligible CPU
- **Qdrant:** ~100 MB RAM base + ~1 MB per 1000 vectors

### Disk

- **Registry databases:** < 1 MB (grows with instance count)
- **Instance data:** varies by conversation volume and skillset
- **Qdrant storage:** proportional to ingested document volume
- **Docker images:** ~500 MB for the Fox image (cached after first pull)

### Instance limits

Set `instances.max_instances` based on available RAM and ports.
A host with 8 GB RAM can comfortably run 10–15 instances. Adjust
based on your Fox image's resource profile.

The port range starts at `instances.port_start` (default 8787) and
allocates sequentially. Ensure the range is open in your firewall
and does not conflict with other services.

---

## Security operations

### Network exposure

| Surface | Default bind | Auth | Notes |
|---------|-------------|------|-------|
| Management panel + API | `127.0.0.1:9090` | Bearer token (`admin_secret`) | Use built-in TLS or a reverse proxy for production |
| Prometheus metrics | `127.0.0.1:9090/metrics` | None | Only active when `metrics.enabled = true`; restrict via firewall or proxy |
| Fox instances | `0.0.0.0:<port>` | Instance-level auth | Ports 8787+ |
| Data plane API | `127.0.0.1:9091` | See data plane docs | Only when `data_plane.enabled = true` |
| Qdrant | `6333` (HTTP), `6334` (gRPC) | None by default | Restrict to localhost or private network |
| Docker socket | Unix socket | Unix permissions | Root-equivalent access |

### Audit checklist

Run periodically (monthly or after significant changes):

- [ ] Admin secret is not the demo value or a weak string
- [ ] Admin secret has been rotated within the last 90 days
- [ ] TLS is active — either built-in (`tls.cert_file` / `tls.key_file`) or via reverse proxy
- [ ] No unnecessary ports are open to the public internet
- [ ] Docker socket is not exposed over TCP
- [ ] fox-control runs as a non-root, dedicated user
- [ ] Data directory permissions are 0700 (owner-only)
- [ ] Qdrant is not accessible from untrusted networks
- [ ] Instance images are pulled from a trusted registry
- [ ] Backups are current and tested

### Incident response

If you suspect the admin secret has been compromised:

1. Rotate the admin secret immediately (see [Secret rotation](#rotate-the-admin-secret)).
2. Review server access logs and reverse proxy logs for unauthorized
   API calls.
3. Check for unauthorized instances: `fox-control list`.
4. Destroy any instances you did not create.
5. If the data plane was accessible, review Qdrant for unauthorized
   data.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| Panel returns 401 | Admin secret mismatch | Verify `FOX_ADMIN_SECRET` matches between client and server. Check env var vs. TOML precedence. |
| fox-control won't start | Missing required secrets | Both `admin_secret` and `instance_password` are required. Set via env vars or TOML. |
| Instance stuck in "provisioning" | Docker pull failure or port conflict | Check Docker daemon is running. Verify the Fox image is pullable. Check no other service is using the allocated port. |
| Instance unhealthy | Container crashed or port unreachable | Check `docker logs <container-id>`. Verify the instance port is not blocked by a firewall. |
| Data plane queries return 503 | Qdrant unreachable | Verify Qdrant is running and ports match the config. Check `docker compose ps` or `systemctl status qdrant`. |
| SSE not working behind proxy | Response buffering enabled | Disable response buffering in your reverse proxy. Caddy handles this automatically. For nginx: `proxy_buffering off;`. |
| Panel shows stale data | SSE disconnected | The panel falls back to 5-second polling. Hard refresh the browser. Check DevTools for EventSource errors. |
| Rollout stopped mid-way | Instance failed health check after update | The failed instance rolls back automatically. Check its logs. Fix the issue, then re-run the rollout. |
| Port conflict on instance start | Another service on the port | Change `instances.port_start` in the config, or stop the conflicting service. |
| Rate limited (429 response) | Client exceeding API rate limit | Check the `Retry-After` header for wait time. Increase `rate_limit.requests_per_second` and `rate_limit.burst` if the load is legitimate. |
| Metrics endpoint returns 404 | Metrics not enabled | Set `metrics.enabled = true` in `fox-control.toml` and restart. |
| Webhook not firing | Target unreachable or event filter mismatch | Verify the webhook URL is reachable from the fox-control host. Check that the `events` filter in the webhook config includes the event type you expect. |
| Auto-restart not working | Feature disabled or misconfigured | Verify `auto_restart.enabled = true` in config. Check `threshold` and `cooldown` values. Review logs for auto-restart decisions. |
| TLS handshake failure | Certificate or key issue | Verify `tls.cert_file` and `tls.key_file` paths exist and are readable by the fox-control user. Check certificate chain completeness and expiry. |
| Backup fails | Destination issue or concurrent backup | Verify the `--output` directory exists and is writable. Ensure no other backup or restore is in progress (check for lock file). |
| Instance stats return 503 | Docker stats unavailable | The container may not be running, or the Docker daemon stats API is unresponsive. Verify the instance is in a healthy or running state. |
| Health history empty | No transitions recorded | The instance has had no health state changes in the last 24 hours, or the event store is not configured. Check `events.db` exists in `data_root`. |

---

## API reference (quick)

### Panel API (port 9090)

All endpoints require `Authorization: Bearer <admin_secret>`
except `/healthz`.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Health check (no auth) |
| `GET` | `/api/instances` | List all instances |
| `GET` | `/api/instances/{id}` | Get instance details |
| `POST` | `/api/instances` | Create an instance |
| `DELETE` | `/api/instances/{id}` | Destroy an instance |
| `GET` | `/api/sources` | List knowledge sources (read-only) |
| `GET` | `/api/sources/{id}` | Get source details (read-only) |
| `GET` | `/api/skillsets` | List skillsets |
| `GET` | `/api/skillsets/{name}` | Get skillset details |
| `POST` | `/api/skillsets` | Upload a skillset (multipart) |
| `GET` | `/api/skillsets/{name}/download` | Download skillset file |
| `DELETE` | `/api/skillsets/{name}` | Delete a skillset |
| `POST` | `/api/query` | Query the knowledge base (proxied to data plane) |
| `GET` | `/api/instances/{id}/stats` | Container resource statistics |
| `GET` | `/api/instances/{id}/health-history` | Health event history (24h) |
| `GET` | `/api/events` | Get recent events |
| `GET` | `/api/events/stream` | SSE event stream |
| `GET` | `/metrics` | Prometheus metrics (no auth, when `metrics.enabled = true`) |

### Data plane admin API (port 9091)

Requires `Authorization: Bearer <admin_secret>`. Only available
when `data_plane.enabled = true`.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/health` | Data plane health (no auth) |
| `GET` | `/v1/sources` | List sources (no auth) |
| `POST` | `/v1/query` | Query vectors (no auth) |
| `GET` | `/v1/admin/sources` | List sources (admin) |
| `POST` | `/v1/admin/sources` | Create a source (JSON) |
| `GET` | `/v1/admin/sources/{id}` | Get source details |
| `DELETE` | `/v1/admin/sources/{id}` | Delete a source |
| `POST` | `/v1/admin/sources/{id}/ingest` | Trigger source ingestion |

### CLI reference (quick)

| Command | Description |
|---------|-------------|
| `fox-control serve` | Start the management plane |
| `fox-control provision --id <id>` | Provision an instance |
| `fox-control destroy --id <id> [--remove-data]` | Destroy an instance |
| `fox-control list` | List all instances |
| `fox-control rollout --image <ref>` | Rolling update (digest reference) |
| `fox-control version` | Print version |
| `fox-control backup --output <dir>` | SQLite VACUUM INTO backup of all databases |
| `fox-control restore --input <dir>` | Restore databases from backup |
| `fox-control diagnostics` | Run 8 built-in health checks |
| `fox-control generate-secret` | Generate a cryptographic secret |
| `fox-control verify <file>` | Verify cosign signature |
| `fox-control conformance run --image <img>` | Run runtime conformance |
| `fox-control conformance plugin --image <img>` | Run plugin conformance |

---

## Known limitations

Fox Fleet has architectural boundaries documented in
[LIMITATIONS.md](../LIMITATIONS.md). Key items for operators:

- **Single-host only** — no multi-node orchestration
- **No high availability** — single process, local SQLite
- **Single admin token** — no user accounts or RBAC
- **Docker socket is root-equivalent** — inherent to the architecture

---

## What's next

- [Configuration Reference](../configuration.md) — full TOML config
  documentation
- [Deployment Guide](../DEPLOYMENT.md) — deployment method details
- [Walkthrough](../WALKTHROUGH.md) — step-by-step screencast script
- [Known Limitations](../LIMITATIONS.md) — architectural boundaries
