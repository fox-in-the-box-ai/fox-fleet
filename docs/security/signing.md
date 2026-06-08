# Release Signing and Verification

Fox Fleet releases are signed using [Sigstore cosign](https://docs.sigstore.dev/cosign/overview/) with keyless signing via GitHub Actions OIDC. Every binary tarball, checksums file, and container image published in a GitHub Release is cryptographically signed.

---

## Chain of trust

1. A maintainer pushes a `v*.*.*` tag to the `fox-fleet` repository.
2. The GitHub Actions release workflow runs in a trusted environment.
3. GitHub provides a short-lived OIDC token attesting that the workflow ran in this repository, from this commit, triggered by this tag.
4. Cosign exchanges the OIDC token for a short-lived signing certificate from [Fulcio](https://docs.sigstore.dev/fulcio/overview/) (Sigstore's certificate authority).
5. Each artifact is signed with the ephemeral key. The certificate, signature, and signing event are recorded in [Rekor](https://docs.sigstore.dev/rekor/overview/) (Sigstore's transparency log).
6. The signatures (`.sig`) and certificates (`.pem`) are published alongside the artifacts in the GitHub Release.

No long-lived signing keys exist. The trust anchor is the GitHub OIDC identity of the release workflow.

---

## Verifying release binaries

### Prerequisites

Install cosign: https://docs.sigstore.dev/cosign/system_config/installation/

### Using `fox-control verify`

```bash
# Download the release tarball, signature, and certificate
# (all three are in the GitHub Release assets)
fox-control verify fox-control-v1.0.0-linux-amd64.tar.gz
```

The command expects `<file>.sig` and `<file>.pem` next to the artifact. Override with flags:

```bash
fox-control verify artifact.tar.gz \
  --signature path/to/artifact.tar.gz.sig \
  --certificate path/to/artifact.tar.gz.pem
```

### Using cosign directly

```bash
cosign verify-blob \
  --signature fox-control-v1.0.0-linux-amd64.tar.gz.sig \
  --certificate fox-control-v1.0.0-linux-amd64.tar.gz.pem \
  --certificate-identity "https://github.com/fox-in-the-box-ai/fox-fleet/.github/workflows/release.yml@refs/tags/v1.0.0" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  fox-control-v1.0.0-linux-amd64.tar.gz
```

Replace `v1.0.0` with the version you downloaded.

### Verifying checksums

The `checksums-sha256.txt` file is also signed:

```bash
cosign verify-blob \
  --signature checksums-sha256.txt.sig \
  --certificate checksums-sha256.txt.pem \
  --certificate-identity "https://github.com/fox-in-the-box-ai/fox-fleet/.github/workflows/release.yml@refs/tags/v1.0.0" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  checksums-sha256.txt
```

Then verify individual files against the checksums:

```bash
sha256sum -c checksums-sha256.txt
```

---

## Verifying container images

Container images pushed to `ghcr.io/fox-in-the-box-ai/fox-control` are signed with cosign keyless signing.

```bash
cosign verify \
  --certificate-identity-regexp "https://github.com/fox-in-the-box-ai/fox-fleet/" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  ghcr.io/fox-in-the-box-ai/fox-control:1.0.0
```

---

## What the signature proves

A valid signature proves:

- The artifact was built by the `release.yml` workflow in the `fox-in-the-box-ai/fox-fleet` repository.
- The workflow was triggered by a specific git tag.
- The build ran on GitHub Actions infrastructure (not on a developer's machine).
- The artifact has not been modified since signing.

A valid signature does **not** prove:

- That the source code is free of vulnerabilities.
- That the tag was created by a specific person (tags are not signed with GPG by default).
- That the binary matches a specific commit (use the SBOM for dependency-level traceability).
