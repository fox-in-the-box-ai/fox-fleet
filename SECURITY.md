# Security Policy

## Reporting a vulnerability

If you discover a security vulnerability in Fox Fleet, please report it responsibly.

**Email:** [security@foxinthebox.io](mailto:security@foxinthebox.io)

Include:

- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Impact assessment (if known)

We will acknowledge receipt within 48 hours and provide an initial assessment within 7 days. Please do not open a public issue for security vulnerabilities.

## Supported versions

| Version | Supported |
|---------|-----------|
| v1.0.x | Yes |
| < v1.0.0 | No (alpha pre-release) |

## Security model

Fox Fleet uses a shared-secret authentication model:

- **`admin_secret`** — authenticates the operator to the dashboard panel and is injected into each instance as `FOX_PLANE_AUTH_SECRET`. Validated via constant-time comparison (`crypto/subtle.ConstantTimeCompare`).
- **`instance_password`** — injected into each instance as `HERMES_WEBUI_PASSWORD`, enabling upstream session authentication. Required by the managed-mode invariant: `FOX_PLANE_AUTH_SECRET` requires upstream auth to be enabled.
- **Fail-loud policy** — `fox-control` refuses to start if either secret is empty.

### Threat model

Fox Fleet targets single-host deployments on trusted networks. The threat model assumes the network between `fox-control` and managed instances is trusted (localhost or private LAN).

For zero-trust deployments, SSO/OIDC integration, mTLS, and edge gateway features, see [Fox Fleet Enterprise](https://github.com/fox-in-the-box-ai).

### What Fleet does not do

- Fleet never modifies Fox instance source code or images
- Fleet never stores credentials in the registry database
- Fleet never sends telemetry or phones home
- Fleet never exposes the Docker socket over the network
