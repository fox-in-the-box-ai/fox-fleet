# Quickstart: Windows

From zero to a running Fox Fleet with one provisioned assistant —
under 15 minutes.

---

## What you'll have at the end

- Fox Fleet running via Docker Desktop + Docker Compose on Windows
- One Fox AI assistant provisioned and accessible in your browser
- The management panel showing instance health

---

## Prerequisites

- **Windows 10/11** (64-bit)
- **Docker Desktop for Windows** —
  [download](https://docs.docker.com/desktop/install/windows-install/).
  WSL 2 backend is enabled by default.
- **Git** — [git-scm.com](https://git-scm.com/download/win) or
  `winget install Git.Git`

After installing Docker Desktop, make sure it's running (whale icon
in the system tray).

---

## Step 1: Get the deployment files

Open **PowerShell** or **Windows Terminal**:

```powershell
git clone https://github.com/fox-in-the-box-ai/fox-fleet.git
cd fox-fleet\deploy\docker-compose
```

---

## Step 2: Configure secrets

```powershell
copy .env.example .env
```

Open `.env` in a text editor (Notepad, VS Code, etc.) and set two
secrets. Use any random 64-character hex strings:

```
FOX_ADMIN_SECRET=<paste-a-random-hex-string-here>
FOX_INSTANCE_PASSWORD=<paste-a-different-random-hex-string-here>
```

Generate random strings in PowerShell:

```powershell
-join ((1..32) | ForEach-Object { '{0:x2}' -f (Get-Random -Maximum 256) })
```

Or use `openssl` from Git Bash: `openssl rand -hex 32`.

Run the command twice — once for each secret. Save the `FOX_ADMIN_SECRET`
value for login.

---

## Step 3: Start the stack

```powershell
docker compose up -d
```

Wait for both services to become healthy:

```powershell
docker compose ps
```

Both `fox-control` and `qdrant` should show `(healthy)`. This takes
about 30–60 seconds on first start.

---

## Step 4: Open the panel

Open `http://localhost:9090` in your browser (Edge, Chrome, Firefox).

Enter your `FOX_ADMIN_SECRET` to log in. The dashboard shows the
Instances tab, Sources tab, and Activity feed.

---

## Step 5: Provision your first Fox

Click **"Create Instance"** in the panel.

Or use PowerShell:

```powershell
$secret = (Get-Content .env | Select-String "FOX_ADMIN_SECRET" | ForEach-Object { $_.Line.Split("=",2)[1] })
curl -X POST http://localhost:9090/api/instances `
  -H "Authorization: Bearer $secret" `
  -H "Content-Type: application/json" `
  -d '{\"id\": \"my-fox\"}'
```

Wait 30–60 seconds for the Fox image to download and health checks
to pass.

---

## Step 6: Open your Fox assistant

Once the instance shows **healthy** in the panel:

```
http://localhost:8787
```

Follow the onboarding wizard to set up your AI provider.

---

## Step 7: Verify

```powershell
curl http://localhost:9090/healthz
# {"status":"ok"}
```

The panel should show `my-fox` with a green health indicator.

---

## Clean up

Destroy the instance via the panel (click the instance → Destroy), or:

```powershell
curl -X DELETE http://localhost:9090/api/instances/my-fox `
  -H "Authorization: Bearer $secret"
```

Stop the stack:

```powershell
docker compose down       # stop services, keep data
docker compose down -v    # deletes qdrant-data only; /var/lib/fox-control persists on the host
```

---

## What's next

- **Add more instances** — provision additional Fox assistants for
  team members
- **Enable the data plane** — edit `fox-control.toml` to configure
  embedding and knowledge sources
- **Production deployment** — for production, deploy on a Linux
  server using [Docker Compose](../DEPLOYMENT.md#docker-compose) or
  [systemd](../DEPLOYMENT.md#systemd-bare-metal)

You now have Fleet running and one Fox provisioned. Continue to the
[Operator Handbook](../operator/handbook.md) for day-2 operations.
