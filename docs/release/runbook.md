# Release Runbook

Step-by-step procedure for cutting a Fox Fleet release.

---

## Prerequisites

- Write access to `fox-in-the-box-ai/fox-fleet`
- Local clone on `main`, up to date
- Go 1.25+ and Docker installed (for local verification)
- `gh` CLI authenticated

## Pre-release checklist

```
[ ] All CI checks green on main
[ ] CHANGELOG.md has a ## [X.Y.Z] - YYYY-MM-DD section with content
[ ] Version bump is correct (semver: breaking=major, feature=minor, fix=patch)
[ ] No open security issues tagged for this release
[ ] Local quality gate passes: make lint && make test && make build
[ ] Conformance suite: trigger workflow_dispatch on main and verify pass (requires Fox instance image; skip with justification if image unavailable — see ADR-0017)
[ ] Per-package coverage: verify no package dropped below its gate (see ADR-0018)
```

---

## 1. Prepare the release

### Update CHANGELOG

Add a new version section to `CHANGELOG.md`. The release workflow extracts
release notes from this section verbatim — empty sections fail the build.

```markdown
## [X.Y.Z] - YYYY-MM-DD

### Added
- ...

### Changed
- ...

### Fixed
- ...
```

Keep the `## [Unreleased]` section above, empty.

### Commit and push

```bash
git add CHANGELOG.md
git commit -m "Prepare vX.Y.Z release"
git push
```

Wait for CI to pass on main before tagging.

---

## 2. Tag and release

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```

The tag push triggers the release workflow (`.github/workflows/release.yml`),
which runs these jobs in order:

```
verify (lint + test + tag/CHANGELOG validation)
  ├── build (5 platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64)
  │     └── release (SBOM, checksums, cosign signing, GitHub Release)
  │           ├── deb-package (amd64 + arm64, stable releases only)
  │           └── homebrew-tap (stable releases only, requires HOMEBREW_TAP_TOKEN)
  └── container (multi-arch Docker image → GHCR)
        └── sign-image (cosign sign + SBOM attestation)
```

### Monitor

```bash
gh run list --workflow=release.yml --limit=1
gh run watch <run-id>
```

### Pre-release tags

Tags containing `-alpha`, `-beta`, or `-rc` are marked as GitHub
pre-releases automatically. Pre-release tags skip Debian packages and
Homebrew tap updates.

---

## 3. Verify the release

### GitHub Release

```bash
gh release view vX.Y.Z
```

Expected assets (23 for stable releases, 21 for pre-releases):

- 5 platform tarballs (`fox-control-vX.Y.Z-{os}-{arch}.tar.gz`)
- 7 cosign signatures (`.sig`) — 5 tarballs + 1 SBOM + 1 checksums
- 7 cosign certificates (`.pem`) — 5 tarballs + 1 SBOM + 1 checksums
- 1 checksums file (`checksums-sha256.txt`)
- 1 CycloneDX SBOM (`.sbom.cdx.json`)
- 2 Debian packages (`fox-control_X.Y.Z_{amd64,arm64}.deb`, stable only)

### Signature verification

```bash
# Download and verify a binary
gh release download vX.Y.Z --pattern 'fox-control-vX.Y.Z-linux-amd64.tar.gz*'
cosign verify-blob \
  --certificate fox-control-vX.Y.Z-linux-amd64.tar.gz.pem \
  --signature fox-control-vX.Y.Z-linux-amd64.tar.gz.sig \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp 'github.com/fox-in-the-box-ai/fox-fleet' \
  fox-control-vX.Y.Z-linux-amd64.tar.gz

# Verify checksums
sha256sum -c checksums-sha256.txt
```

### Container image

```bash
# Verify image exists and is signed
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp 'github.com/fox-in-the-box-ai/fox-fleet' \
  ghcr.io/fox-in-the-box-ai/fox-control:X.Y.Z

# Verify SBOM attestation
cosign verify-attestation \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp 'github.com/fox-in-the-box-ai/fox-fleet' \
  --type cyclonedx \
  ghcr.io/fox-in-the-box-ai/fox-control:X.Y.Z
```

### Binary smoke test

```bash
tar xzf fox-control-vX.Y.Z-linux-amd64.tar.gz
./fox-control-vX.Y.Z-linux-amd64/fox-control version
# Should print: fox-control vX.Y.Z (commit <sha>, built YYYY-MM-DDTHH:MM:SSZ)
```

---

## 4. Post-release

- Update deployment docs if the release changes configuration format
- Notify in the appropriate channels (if configured)
- Close milestone (if using GitHub milestones)

---

## Troubleshooting

### Workflow fails at "Verify CHANGELOG entry exists"

The CHANGELOG section for the version is missing or empty. Add it, commit,
push, delete the tag, and re-tag:

```bash
git tag -d vX.Y.Z
git push origin :refs/tags/vX.Y.Z
# fix CHANGELOG, commit, push
git tag vX.Y.Z
git push origin vX.Y.Z
```

### Workflow fails at "Create release" (release already exists)

If re-tagging after a partial failure, delete the existing release first:

```bash
gh release delete vX.Y.Z --yes --cleanup-tag=false
```

The workflow will create a fresh release on the next run.

### Debian package upload fails

The deb-package job needs `actions/checkout` for `gh` CLI repository context.
If missing, `gh release upload` fails with "not a git repository".

### Container image build fails

Check that `.dockerignore` does not exclude packages imported by
`cmd/fox-control/main.go`. The Docker build uses `CGO_ENABLED=0` for
a pure-Go build (modernc.org/sqlite is CGO-free).

### Homebrew tap not updated

The Homebrew tap update requires `HOMEBREW_TAP_TOKEN` secret to be configured
in the repository settings. The workflow logs a warning and exits cleanly if
the secret is not set.

---

## Secrets required

| Secret | Purpose | Required for |
|--------|---------|--------------|
| `GITHUB_TOKEN` | Auto-provided by GitHub Actions | All jobs |
| `HOMEBREW_TAP_TOKEN` | Push to homebrew-fox-fleet repo | Homebrew tap update (optional) |
