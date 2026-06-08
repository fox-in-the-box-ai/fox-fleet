# Security Advisory 2026-001: Admin Secret Exposed in SSE Query Parameter

**Severity:** High
**CVE:** N/A (pre-disclosure)
**Affected versions:** v0.3.0-alpha through v1.0.0
**Fixed in:** v1.0.1
**Issue:** [#99](https://github.com/fox-in-the-box-ai/fox-fleet/issues/99)
**ADR:** [0012-sse-session-tokens](../architecture/adr/0012-sse-session-tokens.md)

## Summary

The SSE (Server-Sent Events) endpoint `GET /api/events/stream` required authentication via a query parameter (`?token=<admin_secret>`), exposing the admin secret in browser history, server access logs, Referer headers, and any intermediary proxy logs.

## Impact

An attacker with access to browser history, HTTP access logs, or network proxy logs on the same machine or network could extract the admin secret and gain full administrative access to the Fox Fleet management plane.

## Mitigation

Upgrade to v1.0.1. The admin secret is no longer accepted as a query parameter for SSE. Authentication now uses HMAC-SHA256 signed session tokens delivered via HttpOnly cookies.

### Upgrade steps

1. Update `fox-control` to v1.0.1
2. Restart the service
3. Rotate the admin secret (`auth.admin_secret` in config) if it may have been logged
4. Clear browser history entries containing the old `?token=` parameter
5. Rotate any proxy or CDN logs that may contain the query parameter

### New configuration

```toml
[control]
# Session token lifetime for SSE connections (default: 600 seconds)
session_token_ttl_seconds = 600
```

### Breaking changes

- `EventSource` connections using `?token=<secret>` will be rejected (401)
- The panel SPA now obtains a session token via `POST /api/auth/session` before connecting to SSE
- The session token is scoped to `/api/events` and expires after the configured TTL

## Timeline

- 2026-06-08: Issue reported (#99)
- 2026-06-08: ADR published (#104)
- 2026-06-08: Fix implemented and merged (#105)
- 2026-06-08: v1.0.1 released
