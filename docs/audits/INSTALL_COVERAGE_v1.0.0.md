# Install Coverage Audit — v1.0.0

Audit date: 2026-06-08

## Scope

Assessment of every documented and undocumented install path for Fox
Fleet v1.0.0. For each distribution channel, this audit checks whether
installation instructions exist, are complete, are verified, and cover
all applicable OS targets.

---

## Distribution channels

Fox Fleet v1.0.0 ships through 7 install channels and 4 deployment
methods. Some channels deliver the binary; deployment methods configure
how it runs in production.

**Install channels** (how you get the binary):

1. Install script (`install.sh`)
2. Homebrew tap
3. Debian package (`.deb` from GitHub Releases)
4. Binary download (tarball from GitHub Releases)
5. Container image (GHCR)
6. Build from source
7. Docker Compose (bundles container pull + deployment)

**Deployment methods** (how you run it):

1. Docker Compose
2. Helm chart (Kubernetes)
3. systemd service
4. Manual binary

---

## Coverage matrix

For each channel × OS combination:

- **S+D** = Supported and documented (instructions exist, complete, verified)
- **S+P** = Supported and partially documented (works but docs incomplete)
- **S+U** = Supported but undocumented (works but no instructions)
- **N/S** = Not yet supported (gap to document explicitly)
- **N/A** = Not applicable

### Install channels

| Channel | Linux amd64 | Linux arm64 | macOS Intel | macOS Apple Silicon | Windows amd64 |
|---------|-------------|-------------|-------------|---------------------|---------------|
| Install script | S+D | S+D | S+D | S+D | N/S |
| Homebrew | S+P | S+P | S+P | S+P | N/A |
| Debian package | S+P | S+P | N/A | N/A | N/A |
| Binary download | S+D | S+D | S+D | S+D | S+P |
| Container image | S+P | S+P | S+P | S+P | S+P |
| Build from source | S+D | S+D | S+D | S+D | S+U |

### Deployment methods

| Method | Linux amd64 | Linux arm64 | macOS Intel | macOS Apple Silicon | Windows amd64 | K8s |
|--------|-------------|-------------|-------------|---------------------|---------------|-----|
| Docker Compose | S+D | S+D | S+U | S+U | S+U | N/A |
| Helm chart | N/A | N/A | N/A | N/A | N/A | S+D |
| systemd | S+D | S+D | N/A | N/A | N/A | N/A |
| Binary (manual) | S+D | S+D | S+U | S+U | N/S | N/A |

---

## Per-channel assessment

### 1. Install script (`install.sh`)

**Documentation:** `docs/getting-started.md` lines 7–19

**What works:**
- Auto-detects OS (Linux, macOS) and architecture (amd64, arm64)
- Downloads release tarball, verifies SHA-256 checksum
- Installs to `/usr/local/bin` (or `$FOX_INSTALL_DIR`)
- Supports version pinning via `FOX_VERSION`

**What's documented:**
- One-liner install command
- Version pinning syntax
- Brief description of what the script does

**Gaps:**
- No post-install verification command shown (`fox-control version`)
- No cosign signature verification
- Windows explicitly unsupported (script exits with error) — not
  documented as a limitation
- No uninstall instructions
- `sha256sum` used in script — macOS has `shasum -a 256` instead;
  script may fail on stock macOS without coreutils (needs verification)

**Verdict:** Linux fully functional. macOS functional but needs
shasum compatibility check. Windows unsupported — document explicitly.

### 2. Homebrew

**Documentation:** `docs/getting-started.md` lines 23–29

**What works:**
- Tap + install commands provided
- Formula template exists (`deploy/homebrew/fox-control.rb.tmpl`)
- Release workflow auto-generates formula on stable releases
- Platform-specific SHA256 checksums per architecture
- Covers macOS (Intel + Apple Silicon) and Linux (amd64 + arm64)

**What's documented:**
- `brew tap` + `brew install` commands (2 lines)
- Auto-update note

**Gaps:**
- No prerequisites stated (Homebrew itself)
- No post-install verification (`fox-control version`)
- No post-install "what next" guidance
- `HOMEBREW_TAP_TOKEN` secret not documented as required for tap
  updates to work (workflow logs a warning and skips if unset)
- Whether the tap was actually pushed on v1.0.0 release is unverified
  (the Homebrew tap job succeeded but the "Push to tap repository"
  step may have been a no-op if `HOMEBREW_TAP_TOKEN` is not set)

**Verdict:** Partially documented. Install works if tap is populated;
missing verification, prerequisites, and post-install guidance.

### 3. Debian package

**Documentation:** `docs/getting-started.md` lines 34–43

**What works:**
- `.deb` packages built for amd64 and arm64
- Published as GitHub Release assets
- `dpkg -i` install works

**What's documented:**
- Download URL pattern (with VERSION placeholder)
- `dpkg -i` command

**Gaps:**
- No apt repository — users must manually download from GitHub Releases
- VERSION placeholder not filled in with actual version
- No GPG signature verification of the .deb
- No post-install verification
- No systemd service file included in the .deb (the binary installs
  to `/usr/local/bin/` but the user must separately configure systemd
  if they want it as a service)
- No dependency declaration in the .deb control file beyond basic
  metadata
- No uninstall instructions (`dpkg -r fox-control`)

**Verdict:** Partially documented. Works for manual install. Missing
apt repo, verification, service integration, uninstall.

### 4. Binary download

**Documentation:** `docs/getting-started.md` lines 47–66

**What works:**
- 5-platform matrix table (Linux amd64/arm64, macOS Intel/Apple
  Silicon, Windows amd64)
- Tarball naming convention documented
- Extract + install commands shown
- Checksums file mentioned with link to signing docs

**What's documented:**
- Platform matrix table
- tar + install commands (Linux-specific)
- Link to signing verification docs

**Gaps:**
- Extract/install commands assume Linux (`sudo install -m 755`) —
  macOS equivalent not shown
- Windows extraction not documented (no `tar` guidance for Windows,
  no PATH setup)
- Cosign verification referenced but not inline — user must follow
  link to `docs/security/signing.md`
- No post-install verification (`fox-control version`)
- macOS Gatekeeper warning not mentioned (unsigned binary from
  internet will trigger "cannot be opened because the developer
  cannot be verified")

**Verdict:** Linux fully documented. macOS and Windows partially
documented (missing OS-specific commands and quirks).

### 5. Container image

**Documentation:** `docs/getting-started.md` lines 70–76

**What works:**
- Multi-arch image (linux/amd64 + linux/arm64) on GHCR
- Signed with cosign + SBOM attestation
- Tags: `1.0.0`, `1.0`, `sha-<commit>`

**What's documented:**
- `docker pull` command
- Cosign signing mentioned with link to verification docs

**Gaps:**
- No `docker run` example — only `docker pull`
- No tag strategy documented (which tag to use, `latest` vs version)
- No configuration guidance (volume mounts, env vars, port mapping)
- Verification commands not inline
- Docker Desktop on macOS/Windows not mentioned as a prerequisite

**Verdict:** Partially documented. Pull works; run/configure not
covered (deferred to Docker Compose deployment docs).

### 6. Build from source

**Documentation:** `docs/getting-started.md` lines 80–88; README
quickstart lines 92–150

**What works:**
- `git clone` + `make build` documented
- Go 1.25+ prerequisite stated
- Docker prerequisite stated
- Full config template in README quickstart

**What's documented:**
- Clone, build, configure, run, provision, list, destroy
- Data plane config (commented out)

**Gaps:**
- Windows build not mentioned — Go cross-compilation works but
  CGO_ENABLED=0 is required (modernc.org/sqlite is CGO-free but
  build flags may differ)
- No mention of `make install` (doesn't exist)
- No `PATH` setup guidance after build
- README quickstart uses hardcoded `/var/lib/fox-control` which
  requires root on Linux and doesn't exist on macOS/Windows

**Verdict:** Linux/macOS documented and works. Windows undocumented.

### 7. Docker Compose (deployment)

**Documentation:** `docs/DEPLOYMENT.md` lines 34–126;
`docs/WALKTHROUGH.md` (12 scenes)

**What works:**
- Complete 6-step deployment walkthrough
- Secrets generation, config review, env var overrides
- Health check verification via curl
- Panel access instructions
- Instance provisioning example
- TLS via Caddy add-on (separate section)

**What's documented:**
- Linux deployment thoroughly documented
- Prerequisites (Docker, secrets, ports)
- Qdrant sidecar included automatically
- Data plane enabled by default

**Gaps:**
- Says "Fox Fleet runs on any Linux host with Docker" — macOS and
  Windows with Docker Desktop also work but not stated
- No Docker Compose version requirement (v2 syntax used)
- No Docker Desktop setup guidance for macOS/Windows evaluators
- Volume backup procedures not documented
- No upgrade procedure (image tag bump + `docker compose pull`)
- Caddy TLS section is separate and complex

**Verdict:** Linux fully documented. macOS/Windows undocumented but
functional via Docker Desktop.

### 8. Helm chart (deployment)

**Documentation:** `docs/DEPLOYMENT.md` lines 132–203

**What works:**
- Chart located at `deploy/helm/fox-control/`
- Values table with defaults
- Docker socket security implications documented
- External Qdrant requirement documented
- Ingress example provided

**What's documented:**
- Helm install command
- Values reference
- kubectl verification commands
- Port-forward instructions

**Gaps:**
- **Chart version is 0.3.0 / appVersion 0.3.0-alpha** — stale, should
  be 1.0.0 / 1.0.0 for GA
- No RBAC templates (ServiceAccount, ClusterRole, ClusterRoleBinding)
- No NetworkPolicy templates
- No PodDisruptionBudget
- No resource limits validation guidance
- Persistent volume storage class guidance missing
- No Qdrant sidecar option (Compose includes it; Helm requires
  external)
- Tested platforms not listed (Kind, Minikube, EKS, GKE, AKS)

**Verdict:** Documented but chart version is stale. Missing
production-grade K8s resources.

### 9. systemd (deployment)

**Documentation:** `docs/DEPLOYMENT.md` lines 207–277

**What works:**
- Installer script (`deploy/systemd/install.sh`)
- Hardened service file (46 lines, extensive security directives)
- User/group creation automated
- File permissions explicit

**What's documented:**
- Binary build or download
- Installer script usage
- What the installer creates
- Secret injection via env file
- Service start + journalctl
- Security hardening details

**Gaps:**
- macOS not supported (systemd is Linux-only) — not stated
- No firewall configuration (ufw/iptables)
- No SELinux/AppArmor guidance
- No log rotation configuration
- No backup procedure for `/var/lib/fox-control`
- No upgrade procedure

**Verdict:** Linux fully documented. Good security hardening.
Missing operational procedures (backup, upgrade, log rotation).

---

## README install section assessment

**Current structure:** README § "Quickstart" (lines 92–150) — build
from source only. No mention of other install channels. Links to
`docs/getting-started.md` and `docs/DEPLOYMENT.md` are absent from
the quickstart section (only in "Development" section and
walkthrough/deployment docs).

**Problem:** An operator visiting the README sees one install path
(build from source) and no link to the 6 other channels. The "Status"
line mentions "Compose / Helm / systemd / Homebrew / apt / container
image" but none are linked or documented in the README itself.

**Required:** Per the spec, the README install section should be
organized by OS (macOS → Linux → Windows → Kubernetes → Container),
not by channel. Short version in README; deep content in
`docs/install/<os>.md`.

---

## Cross-cutting findings

### F1. No per-OS organization

All docs are organized by channel (install script, Homebrew, Compose,
Helm, systemd). The spec requires organization by OS — what operators
think in terms of.

**Fix:** Create `docs/install/` directory with per-OS files. README
links to each.

### F2. No `docs/install/`, `docs/quickstart/`, or `docs/operator/` directories

These directories don't exist. The spec requires them.

**Fix:** Create all three.

### F3. Helm chart version stale

`deploy/helm/fox-control/Chart.yaml` shows:
```yaml
version: 0.3.0
appVersion: "0.3.0-alpha"
```

Should be `1.0.0` / `1.0.0` for GA.

**Fix:** Update Chart.yaml. This is a code change, not a doc change —
file as part of this PR since it's a stale reference, not a feature.

### F4. Windows support unclear

Windows binary exists in the release (5th platform in the build
matrix). But:
- Install script doesn't support Windows
- Docker Compose docs say "Linux host"
- Build from source doesn't mention Windows
- No Windows-specific guidance anywhere

**Fix:** Document Windows as "supported via Docker Desktop + Docker
Compose" for evaluation, and "binary download available" for manual
use. Note limitations explicitly.

### F5. macOS deployment undocumented

Docker Compose works on macOS via Docker Desktop but docs say "any
Linux host with Docker." macOS operators get no guidance.

**Fix:** Document macOS as a dev/evaluation platform via Docker
Desktop + Docker Compose or Homebrew + manual binary.

### F6. No verification steps for most channels

Only binary download mentions checksums (via link). No channel shows
`fox-control version` as a post-install check.

**Fix:** Every install path ends with a verification step.

### F7. No uninstall instructions for any channel

No channel documents how to remove fox-control.

**Fix:** Add uninstall section per channel.

### F8. Deployment guide assumes Linux exclusively

`docs/DEPLOYMENT.md` line 1: "Fox Fleet runs on any Linux host with
Docker." This excludes macOS and Windows Docker Desktop users who want
to evaluate Fleet.

**Fix:** Expand scope statement to include macOS/Windows for
dev/evaluation.

### F9. `sha256sum` vs `shasum` on macOS

The install script uses `sha256sum` which is a Linux coreutils command.
macOS ships `shasum -a 256` instead. The script may fail on stock
macOS without Homebrew's coreutils.

**Fix:** Verify and fix the install script to handle both.

---

## Action items

| # | Finding | Severity | Fix | Target |
|---|---------|----------|-----|--------|
| F1 | No per-OS doc organization | High | Create docs/install/<os>.md | This PR |
| F2 | Missing directories | High | Create install/, quickstart/, operator/ | This PR |
| F3 | Helm chart version 0.3.0 | Medium | Update Chart.yaml to 1.0.0 | This PR |
| F4 | Windows support unclear | Medium | Document explicitly | This PR |
| F5 | macOS deployment undoc | Medium | Document as dev/eval platform | This PR |
| F6 | No verification steps | Medium | Add per-channel verification | This PR |
| F7 | No uninstall instructions | Low | Add per-channel uninstall | This PR |
| F8 | DEPLOYMENT.md Linux-only | Medium | Expand scope statement | This PR |
| F9 | sha256sum macOS compat | Medium | Fix install.sh or document | This PR |
