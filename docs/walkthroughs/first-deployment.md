# Operator Walkthrough: First Deployment

This walkthrough takes you from zero to a running Fox Fleet with a managed Fox
assistant instance. Estimated time: 10 minutes.

## Prerequisites

- Docker Engine 24.0+ running on the host
- A terminal with `curl` and `docker compose` available
- A Fox container image accessible from the host (public GHCR image or your own)

## 1. Clone and configure

```bash
git clone https://github.com/fox-in-the-box-ai/fox-fleet.git
cd fox-fleet/deploy/docker-compose
cp .env.example .env
```

Generate secrets and add them to `.env`:

```bash
openssl rand -hex 32   # paste as FOX_ADMIN_SECRET
openssl rand -hex 32   # paste as FOX_INSTANCE_PASSWORD
```

The defaults in `fox-control.toml` work out of the box for a single-host
deployment. No changes needed.

## 2. Start the stack

```bash
docker compose up -d
```

Two containers start: `fox-control` (management plane, port 9090) and `qdrant`
(vector database, ports 6333/6334). Both have health checks — `fox-control`
waits for Qdrant before starting.

Verify:

```bash
docker compose ps
curl -s http://localhost:9090/healthz
# {"status":"ok"}
```

## 3. Open the panel

Navigate to `http://localhost:9090` in a browser. You see the login screen:

![Auth screen](../screenshots/auth-light-en-desktop.png)

Enter your `FOX_ADMIN_SECRET` and click **Connect**. The panel loads with an
empty instance list:

![Empty instances](../screenshots/instances-light-en-desktop.png)

## 4. Provision your first instance

Click **Provision Instance**. The modal asks for three fields:

![Provision modal](../screenshots/modal-provision-light-en-desktop.png)

- **Instance ID** (required) — a unique name, e.g. `alice`
- **Skillset** (optional) — select from uploaded skillset definitions
- **Role** (optional) — a principal role for the instance

Click **Create**. Fox Fleet pulls the Fox image (first time only), starts a
container, allocates a port, and registers the instance. The instance card
appears within seconds, showing health status and port.

You can also provision via the API:

```bash
curl -X POST http://localhost:9090/api/instances \
  -H "Authorization: Bearer $FOX_ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"id": "alice"}'
```

## 5. Monitor instance health

Click an instance card to see its detail view. The detail page shows:

- **Status and health** — green dot for healthy, red for unhealthy
- **Port** — the allocated host port for this instance
- **Resource gauges** — CPU, memory, and network I/O (live from Docker stats)
- **Health timeline** — 24-hour health history bar
- **Container logs** — recent stdout/stderr from the Fox container

## 6. Explore other views

### Sources

Navigate to **Sources** in the sidebar. Sources are knowledge collections
ingested into Qdrant via the data plane. If you have the data plane enabled,
sources appear here as they are ingested — each shows document count, chunk
count, and ingestion status.

### Skillsets

Navigate to **Skillsets**. Upload a `.yaml` skillset definition by dragging it
onto the drop zone or clicking **browse**. Skillsets define a Fox instance's
persona, tools, data source bindings, and capabilities. Once uploaded, you can
assign a skillset when provisioning a new instance.

### Query

Navigate to **Query**. This is the knowledge search playground — enter a
natural language query and get ranked results from your ingested sources.
Requires the data plane to be enabled and at least one source ingested.

### Activity

Navigate to **Activity**. The activity feed shows lifecycle events:
provisioning, destruction, health state changes. Events are pushed via
Server-Sent Events (SSE) for real-time updates, with automatic 5-second
polling fallback.

## 7. Customize the UI

The panel supports:

- **Dark mode** — select **Dark** from the theme dropdown in the sidebar footer
- **Locales** — switch between English, Spanish, and French
- **Mobile** — the layout is fully responsive; sidebar collapses to a hamburger
  menu on screens narrower than 768px

![Dark theme](../screenshots/instances-dark-en-desktop.png)

## 8. Destroy an instance

From the instance detail view, click **Destroy Instance**. The confirmation
modal requires you to type the instance ID and optionally check **Remove data**
to delete the instance's data directory. Click **Destroy** to proceed.

## 9. Production hardening

Before running in production, review:

- [Deployment guide](../DEPLOYMENT.md) — Docker Compose, Helm, systemd, and
  binary deployment options
- [Configuration reference](../configuration.md) — full TOML config with TLS,
  rate limiting, auto-restart, and webhooks
- [Operator handbook](../operator/handbook.md) — backup, monitoring, secret
  rotation, troubleshooting
- [Limitations](../LIMITATIONS.md) — known architectural boundaries (single
  host, single auth token, no HA)

## 10. Next steps

- **Add TLS** — see `deploy/caddy/` for automatic Let's Encrypt certificates
- **Upload a skillset** — customize your Fox instances with persona, tools, and
  knowledge bindings
- **Ingest knowledge sources** — point the data plane at your documents for
  RAG-powered responses
- **Set up webhooks** — configure `[[webhooks]]` in the TOML config to receive
  lifecycle event notifications
- **Enable auto-restart** — configure `[auto_restart]` to automatically recover
  unhealthy instances
