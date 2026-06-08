# Minimal Fox-Control Setup

Bare-minimum configuration to run fox-control on a single host.

## Prerequisites

- Go 1.25+ (to build from source) or a pre-built `fox-control` binary
- Docker Engine running and accessible at `/var/run/docker.sock`

## Steps

1. Create the data directory:

   ```bash
   sudo mkdir -p /var/lib/fox-control
   ```

2. Set required secrets:

   ```bash
   export FOX_ADMIN_SECRET="$(openssl rand -hex 24)"
   export FOX_INSTANCE_PASSWORD="$(openssl rand -hex 16)"
   ```

3. Start fox-control:

   ```bash
   fox-control serve --config fox-control.toml
   ```

4. Verify it is running:

   ```bash
   curl http://127.0.0.1:9090/healthz
   ```

## What this does NOT include

- TLS termination (bind to localhost only, or put behind a reverse proxy)
- Qdrant / data plane (no knowledge-base features)
- Webhooks, metrics, or structured logging
