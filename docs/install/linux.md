# Install Fox Fleet on Linux

Fox Fleet runs on Linux (amd64 and arm64) for both production and
development.

---

## Install script (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/fox-in-the-box-ai/fox-fleet/main/install.sh | bash
```

Pin a version:

```bash
FOX_VERSION=1.4.2 curl -fsSL https://raw.githubusercontent.com/fox-in-the-box-ai/fox-fleet/main/install.sh | bash
```

The script auto-detects your architecture, downloads the release
tarball, verifies the SHA-256 checksum, and installs to
`/usr/local/bin`. Override the install directory with
`FOX_INSTALL_DIR`.

Verify:

```bash
fox-control version
# fox-control v1.4.2 (commit ..., built ...)
```

Uninstall:

```bash
sudo rm /usr/local/bin/fox-control
```

---

## Debian / Ubuntu (apt)

Download the `.deb` package from
[GitHub Releases](https://github.com/fox-in-the-box-ai/fox-fleet/releases):

```bash
# amd64
curl -fsSLO https://github.com/fox-in-the-box-ai/fox-fleet/releases/download/v1.4.2/fox-control_1.4.2_amd64.deb
sudo dpkg -i fox-control_1.4.2_amd64.deb

# arm64
curl -fsSLO https://github.com/fox-in-the-box-ai/fox-fleet/releases/download/v1.4.2/fox-control_1.4.2_arm64.deb
sudo dpkg -i fox-control_1.4.2_arm64.deb
```

Verify:

```bash
fox-control version
```

Uninstall:

```bash
sudo dpkg -r fox-control
```

> **Note:** There is no apt repository for automatic updates yet. To
> upgrade, download the new `.deb` and install it over the existing
> version. An apt repository is planned for a future release.

---

## Homebrew

```bash
brew tap fox-in-the-box-ai/fox-fleet
brew install fox-control
```

Verify:

```bash
fox-control version
```

---

## Binary download

Download the tarball for your architecture from
[GitHub Releases](https://github.com/fox-in-the-box-ai/fox-fleet/releases):

| Architecture | File |
|-------------|------|
| x86_64 (amd64) | `fox-control-v1.4.2-linux-amd64.tar.gz` |
| ARM64 (aarch64) | `fox-control-v1.4.2-linux-arm64.tar.gz` |

```bash
tar xzf fox-control-v1.4.2-linux-amd64.tar.gz
sudo install -m 755 fox-control-v1.4.2-linux-amd64/fox-control /usr/local/bin/
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

## Docker Compose

Prerequisites:
- Docker Engine 24+ or Docker Desktop
- Docker Compose v2

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
curl -s http://localhost:9090/healthz
# {"status":"ok"}
```

Open `http://localhost:9090` in your browser.

See the [Deployment Guide](../DEPLOYMENT.md#docker-compose) for the
full walkthrough.

---

## systemd service

For bare-metal or VM deployments with security hardening:

```bash
# Build or download the binary first (see above)
sudo ./deploy/systemd/install.sh ./fox-control

# Set secrets
sudo editor /etc/fox-control/env
# FOX_ADMIN_SECRET=<your-secret>
# FOX_INSTANCE_PASSWORD=<your-password>

# Start
sudo systemctl enable --now fox-control
sudo journalctl -u fox-control -f
```

See the [Deployment Guide](../DEPLOYMENT.md#systemd-bare-metal) for
the full walkthrough including security hardening details.

Uninstall:

```bash
sudo systemctl disable --now fox-control
sudo rm /usr/local/bin/fox-control
sudo rm -r /etc/fox-control
sudo userdel fox-control
# Optionally remove data: sudo rm -r /var/lib/fox-control
```

---

## Build from source

Prerequisites:
- Go 1.25+
- Docker (for integration tests and the Docker plugin)

```bash
git clone https://github.com/fox-in-the-box-ai/fox-fleet.git
cd fox-fleet
make build
./fox-control version
```

Install:

```bash
sudo install -m 755 fox-control /usr/local/bin/
```

---

## RHEL / Fedora / other RPM-based distributions

There is no RPM package or yum/dnf repository. Use one of:
- Install script (recommended)
- Binary download
- Docker Compose
- Build from source

RPM packaging is planned for a future release.

---

## Verify your download (optional)

```bash
# Download the tarball + signature + certificate
gh release download v1.4.2 --repo fox-in-the-box-ai/fox-fleet \
  --pattern 'fox-control-v1.4.2-linux-amd64.tar.gz*'

cosign verify-blob \
  --certificate fox-control-v1.4.2-linux-amd64.tar.gz.pem \
  --signature fox-control-v1.4.2-linux-amd64.tar.gz.sig \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp 'github.com/fox-in-the-box-ai/fox-fleet' \
  fox-control-v1.4.2-linux-amd64.tar.gz
# Verified OK
```

Verify checksums:

```bash
gh release download v1.4.2 --repo fox-in-the-box-ai/fox-fleet \
  --pattern 'checksums-sha256.txt'
sha256sum -c checksums-sha256.txt --ignore-missing
```

See [Release Signing](../security/signing.md) for full details.

---

## Next steps

- [Linux Quickstart](../quickstart/linux.md) — from zero to a running
  Fleet with one provisioned Fox assistant
- [Configuration Reference](../configuration.md) — full config file
  documentation
- [Deployment Guide](../DEPLOYMENT.md) — production deployment options
