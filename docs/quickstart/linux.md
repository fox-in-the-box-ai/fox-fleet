# Quickstart: Linux

From zero to a running Fox Fleet with one provisioned assistant —
under 15 minutes.

---

## What you'll have at the end

- Fox Fleet running via Docker Compose on your Linux machine
- One Fox AI assistant provisioned and accessible in your browser
- The management panel showing instance health

---

## Prerequisites

- **Docker Engine** — [install](https://docs.docker.com/engine/install/)
  for your distribution. Docker Compose v2 is included.
- Your user must be in the `docker` group, or run commands with `sudo`.

Quick check:

```bash
docker info >/dev/null 2>&1 && echo "Docker OK" || echo "Docker not running"
```

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

Generate and set two secrets:

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

Wait for both services to become healthy:

```bash
docker compose ps
```

Both `fox-control` and `qdrant` should show `(healthy)`. This takes
about 30 seconds on first start.

---

## Step 4: Open the panel

Open `http://localhost:9090` in your browser (or use `curl` on a
headless server).

```bash
curl -s http://localhost:9090/healthz
# {"status":"ok"}
```

Enter your `FOX_ADMIN_SECRET` to log in. The dashboard shows the
Instances tab (empty for now), Sources tab, and Activity feed.

---

## Step 5: Provision your first Fox

```bash
curl -X POST http://localhost:9090/api/instances \
  -H "Authorization: Bearer $(grep FOX_ADMIN_SECRET .env | cut -d= -f2)" \
  -H "Content-Type: application/json" \
  -d '{"id": "my-fox"}'
```

Or click **"Create Instance"** in the panel.

Wait 30–60 seconds for Docker to pull the Fox image and for health
checks to pass.

---

## Step 6: Open your Fox assistant

Once the instance shows **healthy**:

```
http://localhost:8787
```

Follow the onboarding wizard to set up your AI provider.

---

## Step 7: Verify

```bash
# Fleet health
curl -s http://localhost:9090/healthz
# {"status":"ok"}

# Instance list
curl -s http://localhost:9090/api/instances \
  -H "Authorization: Bearer $(grep FOX_ADMIN_SECRET .env | cut -d= -f2)"
# Shows my-fox with status "healthy"

# Instance direct access
curl -s http://localhost:8787/health
# Instance health response
```

---

## Clean up

```bash
# Destroy the instance
curl -X DELETE http://localhost:9090/api/instances/my-fox \
  -H "Authorization: Bearer $(grep FOX_ADMIN_SECRET .env | cut -d= -f2)"

# Stop the stack
docker compose down       # keep data
docker compose down -v    # deletes qdrant-data only; /var/lib/fox-control persists on the host
```

---

## Alternative: systemd deployment

For production bare-metal deployments with security hardening:

```bash
# Install the binary
curl -fsSL https://raw.githubusercontent.com/fox-in-the-box-ai/fox-fleet/main/install.sh | bash

# Run the systemd installer
sudo ./deploy/systemd/install.sh /usr/local/bin/fox-control

# Set secrets and start
sudo editor /etc/fox-control/env
sudo systemctl enable --now fox-control
```

See the [systemd deployment guide](../DEPLOYMENT.md#systemd-bare-metal)
for the full walkthrough.

---

## What's next

- **Add more instances** — provision additional Fox assistants for
  team members
- **Enable the data plane** — ingest documents for
  knowledge-augmented responses
- **Add TLS** — see the [Caddy add-on](../DEPLOYMENT.md#6-add-tls-with-caddy-optional)
- **Production hardening** — switch to
  [systemd deployment](../DEPLOYMENT.md#systemd-bare-metal) for
  security hardening, automatic restarts, and dedicated service user

You now have Fleet running and one Fox provisioned. Continue to the
[Operator Handbook](../operator/handbook.md) for day-2 operations.
