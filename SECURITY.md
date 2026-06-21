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
| v1.8.x | Yes |
| v1.7.x | Yes |
| v1.6.x | Yes |
| v1.5.x | Yes |
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

### Cloud mode authentication

When cloud mode is enabled (`[cloud]` config section), Fleet adds per-user authentication for subdomain access:

- **User credentials** — bcrypt (cost 12) password hashing. Passwords stored in the user database, never logged.
- **Session cookies** — `fox_cloud_session`, `HttpOnly`, `Secure`, `SameSite=Lax`, TTL configurable (default 24h).
- **Subdomain login** — password-only form. The username is the subdomain slug (public by design — it's in the URL). Authentication relies solely on password strength and rate limiting.
- **Login rate limiting** — token bucket, default 5 requests/minute. Applies globally across all login endpoints (root domain + all subdomains). Configurable via `login_rate_limit` in `[cloud]` config.
- **Session isolation** — sessions are scoped to their subdomain. A session for `alice.fleet.example.com` cannot access `bob.fleet.example.com`.

### Cloud mode accepted risks

- **Global rate limiter** — the login rate limiter is a single shared bucket, not per-subdomain or per-IP. At current scale (single-digit users), this is adequate. At larger scale, an attacker brute-forcing one subdomain exhausts the budget for all users. Per-IP rate limiting (requiring X-Forwarded-For parsing behind a reverse proxy) is tracked for a future release.
- **No account lockout** — failed login attempts are rate-limited but do not lock accounts. Lockout requires a persistent failure counter per user, tracked for a future release.
- **Username enumeration** — subdomain slugs are public by design (they're in DNS and the URL). The password is the sole authentication factor. Error messages are generic ("invalid credentials") as defense in depth only.

### Threat model

Fox Fleet targets single-host deployments on trusted networks. The threat model assumes the network between `fox-control` and managed instances is trusted (localhost or private LAN).

For zero-trust deployments, SSO/OIDC integration, mTLS, and edge gateway features, see [Fox Fleet Enterprise](https://github.com/fox-in-the-box-ai).

### What Fleet does not do

- Fleet never modifies Fox instance source code or images
- Fleet stores per-instance query tokens and HMAC signing keys in the registry database — these are operational credentials, not the admin secret itself
- Fleet never sends telemetry or phones home
- Fleet never exposes the Docker socket over the network
