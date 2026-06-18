# ADR 0019: Subdomain-per-Instance Routing

**Date:** 2026-06-18
**Status:** Accepted
**Deciders:** Dennis Vorobyov (founder)

## Context

Fox Fleet's cloud mode (v1.5.0) introduced multi-user provisioning with login sessions. Users accessed their Fox instances through a reverse-proxy path under the base domain (`/cloud/<instance-id>/`). This required URL rewriting to strip the path prefix before forwarding to Fox, which broke Fox's assumption that it serves at root — `Location` headers, relative asset paths, and WebSocket upgrades all assumed `/` as the base path.

Three approaches were evaluated during architecture review:

1. **Subdomain-per-instance** — each user gets `<username>.<domain>`, Fox serves at root, no URL rewriting.
2. **Path-prefix with `X-Ingress-Path`** — inject a base-path header; Fox rewrites internally. Requires upstream changes.
3. **Iframe embed** — wrap Fox in an iframe on the base domain. Breaks clipboard, auth, and accessibility.

## Decision

Subdomain-per-instance routing. The slug is the cloud username, validated as a DNS label (`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`).

Key design choices:

- **Host dispatcher** routes by `Host` header at the top-level HTTP handler — base domain to existing mux, subdomains to instance proxy.
- **Cookie `Domain` attribute left empty** — browser defaults to exact-host-only scoping, preventing cookie leakage across subdomains.
- **Per-instance CSPRNG secrets** — each provisioned instance receives unique `PlaneAuthToken` and `InstancePassword` generated at provision time, replacing shared secrets.
- **Caddy `on_demand_tls`** — wildcard DNS + per-subdomain certificate issuance, validated by a loopback-restricted `/cloud/tls-check` endpoint.
- **Per-subdomain login flow** — `<slug>.<domain>/login` with username-slug enforcement (you can only log in as the user whose subdomain you're on).
- **`SameSite=Strict` retained** — subdomains share the same registrable domain (RFC 6265bis), so cookies are sent on navigation within the site.

## Consequences

- Fox receives requests at root with no path rewriting — all Fox features work unmodified.
- Requires wildcard DNS (`*.<domain>`) pointed at the Fleet host.
- TLS certificate management moves from a single wildcard cert to Caddy's on-demand issuance.
- Legacy `/cloud/*` proxy paths return 301 redirects to `/admin/` for backwards compatibility.
- The SSE token-via-query-parameter path is removed — tokens are now transmitted only in headers.
