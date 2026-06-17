# Install Fox Fleet on Windows

Fox Fleet runs on Windows via Docker Desktop for development and
evaluation. Production deployments should use Linux.

---

## Docker Compose via Docker Desktop (recommended)

This is the primary install path on Windows.

### Prerequisites

1. **Docker Desktop for Windows** — download from
   [docker.com](https://docs.docker.com/desktop/install/windows-install/).
   Ensure WSL 2 backend is enabled (Docker Desktop enables this by
   default on Windows 10/11).
2. **Git** — [git-scm.com](https://git-scm.com/download/win) or
   `winget install Git.Git`.

### Install

Open PowerShell or Windows Terminal:

```powershell
git clone https://github.com/fox-in-the-box-ai/fox-fleet.git
cd fox-fleet\deploy\docker-compose
copy .env.example .env
```

Edit `.env` and set your secrets:

```
FOX_ADMIN_SECRET=<generate-a-random-32-char-hex-string>
FOX_INSTANCE_PASSWORD=<generate-a-random-32-char-hex-string>
```

Generate random secrets in PowerShell:

```powershell
-join ((1..32) | ForEach-Object { '{0:x2}' -f (Get-Random -Maximum 256) })
```

Or use `openssl` from Git Bash (included with Git for Windows):

```bash
openssl rand -hex 32
```

Run the command twice — once for each secret.

Start the stack:

```powershell
docker compose up -d
```

### Verify

```powershell
docker compose ps
# Both fox-control and qdrant should show "healthy"

curl http://localhost:9090/healthz
# {"status":"ok"}
```

Open `http://localhost:9090` in your browser.

### Stop

```powershell
docker compose down            # stop services, keep data
docker compose down -v         # stop services AND delete volumes
```

---

## Binary download

A Windows binary (`fox-control-v1.5.0-windows-amd64.tar.gz`) is
available from [GitHub Releases](https://github.com/fox-in-the-box-ai/fox-fleet/releases).

```powershell
# Extract
tar xzf fox-control-v1.5.0-windows-amd64.tar.gz

# Run
.\fox-control-v1.5.0-windows-amd64\fox-control.exe version
```

### Windows SmartScreen

The binary is not code-signed. Windows SmartScreen may show "Windows
protected your PC." Click "More info" → "Run anyway."

### Limitations of the Windows binary

The Windows binary can connect to Docker Desktop's Docker daemon, but:
- Volume paths use Windows conventions (backslashes, drive letters)
- The Docker socket path differs from Linux
  (`//./pipe/docker_engine` instead of `/var/run/docker.sock`)
- Config file paths in `fox-control.toml` must use Windows paths or
  forward slashes

Docker Compose is the recommended path on Windows because it handles
these differences automatically.

---

## Build from source (WSL 2)

If you prefer building from source, use WSL 2 for a Linux-native
build environment:

```bash
# Inside WSL 2 (Ubuntu)
sudo apt update && sudo apt install -y golang docker.io
git clone https://github.com/fox-in-the-box-ai/fox-fleet.git
cd fox-fleet
make build
./fox-control version
```

The WSL 2 binary runs natively in the WSL environment and can access
Docker Desktop's daemon if WSL integration is enabled in Docker
Desktop settings.

---

## What's not supported on Windows

- **Install script** — the `install.sh` script requires bash and is
  Linux/macOS only.
- **systemd** — Linux-only process manager.
- **Homebrew** — macOS/Linux only.
- **Debian packages** — Linux-only.

---

## Next steps

- [Windows Quickstart](../quickstart/windows.md) — from zero to a
  running Fleet with one provisioned Fox assistant
- [Configuration Reference](../configuration.md) — full config file
  documentation
- [Deployment Guide](../DEPLOYMENT.md) — production deployment options
