# Production Fox-Control Deployment

Docker Compose stack with TLS, Qdrant, the data plane, structured logging,
metrics, and webhook notifications.

## Prerequisites

- Docker Engine 24+ with Compose v2
- A TLS certificate and key (see step 2)
- An embedding provider accessible from the host (Ollama, OpenAI-compatible API, etc.)

## Steps

1. Copy the environment template and fill in secrets:

   ```bash
   cp .env.example .env
   # Edit .env — set FOX_ADMIN_SECRET (min 16 chars) and FOX_INSTANCE_PASSWORD.
   ```

2. Place your TLS certificate and key:

   ```bash
   mkdir -p tls
   cp /path/to/cert.pem tls/tls.crt
   cp /path/to/key.pem  tls/tls.key
   chmod 600 tls/tls.key
   ```

3. Review `fox-control.toml`. In particular:

   - `[docker] image` — pin to a digest for reproducible deploys.
   - `[embedding]` — adjust `base_url` and `model` for your provider.
   - `[[webhooks]]` — set the URL and secret for your webhook receiver.

4. Start the stack:

   ```bash
   docker compose up -d
   ```

5. Verify health:

   ```bash
   curl -k https://localhost:9090/healthz
   ```

## Architecture

```
 Internet
    |
    v
 [fox-control :9090] --TLS--> clients
    |
    +-- Docker socket --> provisions Fox instances (:8787–:8796)
    |
    +-- [qdrant :6333/6334] (internal network only)
    |
    +-- [data-plane :9091] --> embedding provider
```

## Notes

- Qdrant ports are **not** exposed to the host by default. Uncomment the
  `ports` block in `docker-compose.yml` if you need direct access for debugging.
- Log rotation is configured at 10 MB / 3 files per container.
- The `/metrics` endpoint is enabled for Prometheus scraping.
