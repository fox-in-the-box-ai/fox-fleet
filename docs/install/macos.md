# Install Fox Fleet on macOS

Fox Fleet runs on macOS (Apple Silicon and Intel) for development and
evaluation. For production deployments, use Linux.

---

## Homebrew (recommended)

```bash
brew tap fox-in-the-box-ai/fox-fleet
brew install fox-control
```

Verify:

```bash
fox-control version
# fox-control v1.5.0 (commit ..., built ...)
```

Upgrade:

```bash
brew upgrade fox-control
```

Uninstall:

```bash
brew uninstall fox-control
brew untap fox-in-the-box-ai/fox-fleet
```

---

## Install script

```bash
curl -fsSL https://raw.githubusercontent.com/fox-in-the-box-ai/fox-fleet/main/install.sh | bash
```

Pin a version:

```bash
FOX_VERSION=1.5.0 curl -fsSL https://raw.githubusercontent.com/fox-in-the-box-ai/fox-fleet/main/install.sh | bash
```

The script detects your architecture (Apple Silicon or Intel),
downloads the release tarball, verifies the SHA-256 checksum, and
installs to `/usr/local/bin`.

Verify:

```bash
fox-control version
```

Uninstall:

```bash
sudo rm /usr/local/bin/fox-control
```

---

## Binary download

Download the tarball for your architecture from
[GitHub Releases](https://github.com/fox-in-the-box-ai/fox-fleet/releases):

| Architecture | File |
|-------------|------|
| Apple Silicon (M1/M2/M3/M4) | `fox-control-v1.5.0-darwin-arm64.tar.gz` |
| Intel | `fox-control-v1.5.0-darwin-amd64.tar.gz` |

```bash
# Apple Silicon example
tar xzf fox-control-v1.5.0-darwin-arm64.tar.gz
sudo install -m 755 fox-control-v1.5.0-darwin-arm64/fox-control /usr/local/bin/
```

### macOS Gatekeeper

The binary is not notarized by Apple. On first run, macOS may show
"cannot be opened because the developer cannot be verified." To allow
it:

```bash
# Option 1: Remove the quarantine attribute
xattr -d com.apple.quarantine /usr/local/bin/fox-control

# Option 2: Right-click → Open in Finder, then confirm
```

Verify:

```bash
fox-control version
```

Uninstall:

```bash
sudo rm /usr/local/bin/fox-control
```

---

## Docker Compose (for evaluation)

If you have Docker Desktop installed, Docker Compose is the fastest
path to a running Fleet with the data plane included.

Prerequisites:
- [Docker Desktop for Mac](https://docs.docker.com/desktop/install/mac-install/)

```bash
cd deploy/docker-compose
cp .env.example .env

# Set secrets
cat > .env <<EOF
FOX_ADMIN_SECRET=$(openssl rand -hex 32)
FOX_INSTANCE_PASSWORD=$(openssl rand -hex 32)
EOF

docker compose up -d
```

Verify:

```bash
docker compose ps          # both services should be "healthy"
curl http://localhost:9090/healthz
# {"status":"ok"}
```

Open `http://localhost:9090` in your browser.

See the [Deployment Guide](../DEPLOYMENT.md#docker-compose) for the
full walkthrough including TLS, data plane configuration, and
production settings.

---

## Build from source

Prerequisites:
- Go 1.25+
- Docker Desktop (for integration tests and the Docker plugin)

```bash
git clone https://github.com/fox-in-the-box-ai/fox-fleet.git
cd fox-fleet
make build
./fox-control version
```

The binary is at `./fox-control`. Move it to your PATH:

```bash
sudo install -m 755 fox-control /usr/local/bin/
```

---

## Verify your download (optional)

Verify the binary's cosign signature against the GitHub Actions OIDC
identity:

```bash
# Download the tarball + signature + certificate
gh release download v1.5.0 --repo fox-in-the-box-ai/fox-fleet \
  --pattern 'fox-control-v1.5.0-darwin-arm64.tar.gz*'

cosign verify-blob \
  --certificate fox-control-v1.5.0-darwin-arm64.tar.gz.pem \
  --signature fox-control-v1.5.0-darwin-arm64.tar.gz.sig \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp 'github.com/fox-in-the-box-ai/fox-fleet' \
  fox-control-v1.5.0-darwin-arm64.tar.gz
# Verified OK
```

See [Release Signing](../security/signing.md) for full details.

---

## Next steps

- [macOS Quickstart](../quickstart/macos.md) — from zero to a running
  Fleet with one provisioned Fox assistant
- [Configuration Reference](../configuration.md) — full config file
  documentation
- [Deployment Guide](../DEPLOYMENT.md) — production deployment options
