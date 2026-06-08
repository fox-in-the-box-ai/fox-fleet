# Installation

Fox Fleet is distributed as a single `fox-control` binary. Choose the method that fits your environment.

---

## Install script (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/fox-in-the-box-ai/fox-fleet/main/install.sh | bash
```

Pin a specific version:

```bash
FOX_VERSION=x.y.z curl -fsSL https://raw.githubusercontent.com/fox-in-the-box-ai/fox-fleet/main/install.sh | bash
```

The script auto-detects OS and architecture, downloads the release tarball, verifies the SHA-256 checksum, and installs to `/usr/local/bin`. Override the install directory with `FOX_INSTALL_DIR`.

---

## Homebrew

```bash
brew tap fox-in-the-box-ai/fox-fleet
brew install fox-control
```

The tap is updated automatically on each stable release.

---

## Debian / Ubuntu

Download the `.deb` from the [GitHub Releases](https://github.com/fox-in-the-box-ai/fox-fleet/releases) page:

```bash
curl -fsSLO https://github.com/fox-in-the-box-ai/fox-fleet/releases/latest/download/fox-control_VERSION_amd64.deb
sudo dpkg -i fox-control_VERSION_amd64.deb
```

Replace `VERSION` and `amd64` with the desired version and architecture (`amd64` or `arm64`).

---

## Binary download

Download a pre-built tarball from [GitHub Releases](https://github.com/fox-in-the-box-ai/fox-fleet/releases):

| Platform | Architecture | File |
|----------|-------------|------|
| Linux | x86_64 | `fox-control-v{version}-linux-amd64.tar.gz` |
| Linux | ARM64 | `fox-control-v{version}-linux-arm64.tar.gz` |
| macOS | Intel | `fox-control-v{version}-darwin-amd64.tar.gz` |
| macOS | Apple Silicon | `fox-control-v{version}-darwin-arm64.tar.gz` |
| Windows | x86_64 | `fox-control-v{version}-windows-amd64.tar.gz` |

Extract and move to your PATH:

```bash
tar xzf fox-control-vVERSION-linux-amd64.tar.gz
sudo install -m 755 fox-control-vVERSION-linux-amd64/fox-control /usr/local/bin/
```

Verify the download with the signed checksums file. See [Release Signing](security/signing.md) for details.

---

## Container image

```bash
docker pull ghcr.io/fox-in-the-box-ai/fox-control:latest
```

All container images are signed with Sigstore cosign. See [Release Signing](security/signing.md) for verification.

---

## Build from source

Requires Go 1.25+ and Docker.

```bash
git clone https://github.com/fox-in-the-box-ai/fox-fleet.git
cd fox-fleet
make build
```

---

## Verify installation

```bash
fox-control version
```

---

## Next steps

- [Walkthrough](WALKTHROUGH.md) — step-by-step from first launch to teardown
- [Deployment Guide](DEPLOYMENT.md) — production deployment with Docker Compose, Helm, or systemd
- [Configuration Reference](configuration.md) — full config file documentation
