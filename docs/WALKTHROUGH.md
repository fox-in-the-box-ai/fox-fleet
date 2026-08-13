# Fox Fleet Walkthrough

A step-by-step walkthrough of Fox Fleet from first launch to a running fleet of Fox AI assistants. Use this as a screencast script or follow along in your terminal.

**Prerequisites:** Docker running, Git installed, Go 1.25+ (for building from source).

---

## Scene 1: Start the stack

Start Fox Fleet with Docker Compose. This brings up the management plane and a Qdrant vector database.

```bash
cd deploy/docker-compose
cp .env.example .env

# Set real secrets (or use these for the demo)
cat > .env <<EOF
FOX_ADMIN_SECRET=demo-secret-do-not-use-in-production
FOX_INSTANCE_PASSWORD=demo-password-do-not-use-in-production
EOF

docker compose up -d
```

Wait for health checks:

```bash
docker compose ps
# Both fox-control and qdrant should show "healthy"
```

Verify the API responds:

```bash
curl http://localhost:9090/healthz
# {"status":"ok"}
```

---

## Scene 2: Log in to the panel

Open `http://localhost:9090` in your browser.

**What you see:** The auth screen with Fox Fleet branding, a tagline, and a
single password field for the admin secret.

![Auth screen](screenshots/auth-light-en-desktop.png)

1. Paste your `FOX_ADMIN_SECRET` (`demo-secret-do-not-use-in-production` if using demo values).
2. Click **Connect**.

**What you see next:** The app shell with a sidebar containing six navigation
items in three sections — **Manage** (Instances, Sources, Skillsets),
**Observe** (Query, Activity), and **Configure** (Settings). The Instances
view is active and shows an empty state — no Fox instances running yet.

![Empty instances](screenshots/instances-light-en-desktop.png)

---

## Scene 3: Provision your first Fox instance

1. Click **Provision Instance** (or **Create Instance** in the empty state).
2. In the modal, enter an instance ID: `fox-1`.
3. (Optional) Select a skillset and role if you have them configured.
4. Click **Create**.

![Provision modal](screenshots/modal-provision-light-en-desktop.png)

**What happens behind the scenes:** fox-control allocates port 8787, writes instance config files (`config.yaml`, `settings.json`, `hermes.env`), pulls the Fox image if not cached, and starts a Docker container.

**What you see:** The activity feed at the bottom updates in real time (via Server-Sent Events) showing the provisioning lifecycle:

```
provision  fox-1  provisioning started
provision  fox-1  container created
provision  fox-1  health check passed
```

The instance card on the dashboard turns green when the health check passes.

---

## Scene 4: Explore instance details

1. Click the **fox-1** card on the dashboard.

**What you see:** A detail view with:
- Instance ID, status (running), allocated port (8787)
- Container image and tag
- Health status with last-check timestamp
- Created-at timestamp
- Assigned skillset and role (if configured)

The detail view auto-updates — if the instance health changes, you see it within seconds.

---

## Scene 5: Provision a second instance

1. Go back to the Instances view.
2. Click **Provision Instance** again.
3. ID: `fox-2`.
4. Click **Create**.

**What you see:** A second instance card appears. Port 8788 is allocated automatically. Both instance cards show green health status.

The activity feed shows both provisioning events interleaved.

---

## Scene 6: Ingest a knowledge source (data plane)

If the data plane is enabled (default in the Docker Compose stack), you can ingest documents for your Fox instances to query.

1. Switch to **Sources** in the sidebar (under **Manage**).
2. Click a source to see its details — status, document count, last ingestion time.

Via API (since file upload requires curl):

```bash
# Create a test document
echo "Fox Fleet manages AI assistant instances on Docker." > /tmp/test-doc.txt

# Ingest it
curl -X POST http://localhost:9090/api/sources \
  -H "Authorization: Bearer demo-secret-do-not-use-in-production" \
  -F "file=@/tmp/test-doc.txt" \
  -F "name=test-knowledge"
```

The Sources tab updates to show the new source with its ingestion status.

---

## Scene 7: Query the knowledge base

1. Switch to **Query** in the sidebar (under **Observe**).

Via API:

```bash
curl -X POST http://localhost:9090/api/query \
  -H "Authorization: Bearer demo-secret-do-not-use-in-production" \
  -H "Content-Type: application/json" \
  -d '{"query": "What does Fox Fleet do?", "top_k": 3}'
```

**What you see:** Relevant chunks from your ingested documents, ranked by similarity score.

---

## Scene 8: Manage skillsets

1. Switch to **Skillsets** in the sidebar (under **Manage**).

**What you see:** A list of uploaded skillset manifests. Each skillset defines the tools and capabilities available to Fox instances.

To upload a skillset:

```bash
curl -X POST http://localhost:9090/api/skillsets \
  -H "Authorization: Bearer demo-secret-do-not-use-in-production" \
  -F "file=@/path/to/skillset.yaml"
```

Skillsets appear in the create-instance dialog and can be assigned to new instances.

---

## Scene 9: Dark mode and language

1. In the sidebar footer, change the **Theme** dropdown from **System** to **Dark**.

**What you see:** The entire panel switches to dark mode — dark background,
light text, the Fox palette adapts. The preference is saved in your browser.

![Dark theme](screenshots/instances-dark-en-desktop.png)

2. Change the **Language** dropdown to **Español**.

**What you see:** All UI text switches to Spanish. Navigation labels, buttons, table headers, status badges, toast messages — everything updates. Switch back to English anytime.

---

## Scene 10: Mobile view

Resize your browser window to a narrow width (below 768px), or open DevTools and toggle the device toolbar.

**What you see:**
- The sidebar collapses behind a hamburger menu (top-left).
- Cards stack in a single column.
- Tables become scrollable.
- The detail views and modals adapt to full-width.

Tap the hamburger to slide the sidebar in as an overlay. Tap the overlay or
press Escape to close it.

![Mobile view](screenshots/instances-light-en-mobile.png)
![Sidebar open on mobile](screenshots/sidebar-open-light-en-mobile.png)

---

## Scene 11: Real-time updates

Open two browser tabs to `http://localhost:9090`, both logged in.

In Tab 1, destroy an instance:
1. Click **fox-2** → **Destroy**.
2. Confirm.

**Watch Tab 2:** The instance card disappears and the activity feed updates within 1 second — no page refresh needed. This is Server-Sent Events in action.

---

## Scene 12: Destroy and clean up

Destroy remaining instances:

```bash
curl -X DELETE http://localhost:9090/api/instances/fox-1 \
  -H "Authorization: Bearer demo-secret-do-not-use-in-production"
```

Stop the stack:

```bash
cd deploy/docker-compose
docker compose down       # keeps data
# docker compose down -v  # deletes qdrant-data only; /var/lib/fox-control persists
```

---

## What's next

- **Production deployment:** See [DEPLOYMENT.md](DEPLOYMENT.md) for Helm, systemd, and TLS setup.
- **Rolling updates:** Use `fox-control rollout --image <digest>` to update all instances with health-gated rollback.
- **Limitations:** See [LIMITATIONS.md](LIMITATIONS.md) for architectural boundaries and known gaps.
