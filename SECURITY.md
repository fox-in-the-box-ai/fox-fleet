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
| v1.4.x | Yes |
| v1.3.x | Yes |
| v1.2.x | Yes |
| v1.1.x | Yes |
| v1.0.x | Yes |
| < v1.0.0 | No (alpha pre-release) |

## Security model

Fox Fleet uses a layered authentication model:

- **`admin_secret`** — authenticates the operator to the dashboard panel and is injected into each instance as `FOX_PLANE_AUTH_SECRET`. Validated via constant-time comparison (`crypto/subtle.ConstantTimeCompare`).
- **Session tokens (SSE)** — HMAC-SHA256 signed, short-lived tokens for Server-Sent Events connections. The admin secret is used once to obtain a session token; the session token is what appears in URLs. Signing keys are stored in the registry database.
- **Per-instance query tokens** — each instance receives a unique 32-byte token for data plane query authentication, stored in the registry database alongside the instance record.
- **`instance_password`** — injected into each instance as `HERMES_WEBUI_PASSWORD`, enabling upstream session authentication. Required by the managed-mode invariant: `FOX_PLANE_AUTH_SECRET` requires upstream auth to be enabled.
- **Fail-loud policy** — `fox-control` refuses to start if either secret is empty. Admin secret must be at least 16 characters.

### Threat model

Fox Fleet targets single-host deployments on trusted networks. The threat model assumes the network between `fox-control` and managed instances is trusted (localhost or private LAN).

For zero-trust deployments, SSO/OIDC integration, mTLS, and edge gateway features, see [Fox Fleet Enterprise](https://github.com/fox-in-the-box-ai).

### What Fleet does not do

- Fleet never modifies Fox instance source code or images
- Fleet stores per-instance query tokens and HMAC signing keys in the registry database — these are operational credentials, not the admin secret itself
- Fleet never sends telemetry or phones home
- Fleet never exposes the Docker socket over the network
