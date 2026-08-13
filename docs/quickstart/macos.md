# Quickstart: macOS

From zero to a running Fox Fleet with one provisioned assistant —
under 15 minutes.

---

## What you'll have at the end

- Fox Fleet running via Docker Compose on your Mac
- One Fox AI assistant provisioned and accessible in your browser
- The management panel showing instance health

---

## Prerequisites

- **Docker Desktop for Mac** — [download](https://docs.docker.com/desktop/install/mac-install/).
  Make sure it's running (whale icon in the menu bar).

---

## Step 1: Get the deployment files

```bash
git clone https://github.com/fox-in-the-box-ai/fox-fleet.git
cd fox-fleet/deploy/docker-compose
```

---

## Step 2: Configure secrets

```bash
cp .env.example .env
```

Edit `.env` and set two secrets:

```bash
cat > .env <<EOF
FOX_ADMIN_SECRET=$(openssl rand -hex 32)
FOX_INSTANCE_PASSWORD=$(openssl rand -hex 32)
EOF
```

Save the `FOX_ADMIN_SECRET` value — you'll need it to log into the
panel.

---

## Step 3: Start the stack

```bash
docker compose up -d
```

This starts two services:
- **fox-control** — management plane (port 9090)
- **qdrant** — vector database for the knowledge data plane

Wait for both to become healthy:

```bash
docker compose ps
```

Both should show `(healthy)` in the STATUS column. This takes about
30 seconds on first start (Docker pulls the images).

---

## Step 4: Open the panel

Open `http://localhost:9090` in your browser.

You'll see the Fox Fleet dashboard. Enter your `FOX_ADMIN_SECRET`
value to log in.

The dashboard shows:
- **Instances** tab — empty for now
- **Sources** tab — knowledge data plane sources (if configured)
- **Activity feed** — real-time lifecycle events via SSE

---

## Step 5: Provision your first Fox

Click **"Create Instance"** in the panel, or use the API:

```bash
curl -X POST http://localhost:9090/api/instances \
  -H "Authorization: Bearer $(grep FOX_ADMIN_SECRET .env | cut -d= -f2)" \
  -H "Content-Type: application/json" \
  -d '{"id": "my-fox"}'
```

The panel shows the instance transitioning through:
provisioning → starting → healthy.

This takes 30–60 seconds on first run (Docker pulls the Fox image).

---

## Step 6: Open your Fox assistant

Once the instance shows **healthy**, open the Fox URL in your browser:

```
http://localhost:8787
```

You'll see the Fox onboarding wizard. Follow the steps to set up your
AI provider (or use a local Ollama model if you have one running).

---

## Step 7: Verify everything works

```bash
# Fleet health
curl -s http://localhost:9090/healthz | python3 -m json.tool
# {"status": "ok"}

# Instance status
curl -s http://localhost:9090/api/instances \
  -H "Authorization: Bearer $(grep FOX_ADMIN_SECRET .env | cut -d= -f2)" | python3 -m json.tool
# Shows my-fox with status "healthy"
```

---

## Clean up

Destroy the instance:

```bash
curl -X DELETE http://localhost:9090/api/instances/my-fox \
  -H "Authorization: Bearer $(grep FOX_ADMIN_SECRET .env | cut -d= -f2)"
```

Stop the stack:

```bash
docker compose down       # keep data
docker compose down -v    # deletes qdrant-data only; /var/lib/fox-control persists on the host
```

---

## What's next

- **Add more instances** — provision additional Fox assistants for your
  team members, each on its own port
- **Enable the data plane** — ingest organizational documents for
  knowledge-augmented responses. Edit `fox-control.toml` to configure
  embedding and sources.
- **Add TLS** — see the [Caddy add-on](../DEPLOYMENT.md#6-add-tls-with-caddy-optional)
  for automatic HTTPS via Let's Encrypt
- **Production deployment** — for a production Linux server, see the
  [systemd deployment guide](../DEPLOYMENT.md#systemd-bare-metal)

You now have Fleet running and one Fox provisioned. Continue to the
[Operator Handbook](../operator/handbook.md) for day-2 operations.
