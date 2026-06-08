# ADR 0012: Short-lived session tokens for SSE

**Status:** Accepted  
**Date:** 2026-06-08  
**Issue:** [#99](https://github.com/fox-in-the-box-ai/fox-fleet/issues/99)

## Context

The browser `EventSource` API does not support custom HTTP headers.
The panel SPA currently passes the admin secret as a URL query
parameter when connecting to the SSE endpoint:

```javascript
new EventSource("/api/events/stream?token=" + encodeURIComponent(state.secret))
```

The admin secret then appears in:

- Server access logs (`GET /api/events/stream?token=<secret>`)
- Reverse proxy logs (nginx, Caddy, cloud LB)
- Browser history and DevTools network tab
- Screen recordings and screenshots
- Error tracking tools that capture request URLs

Any of these surfaces being shared externally is a full admin
compromise — the token is static and grants full API access.

## Decision

Introduce short-lived, purpose-scoped session tokens for SSE. The
admin secret is used once (server-to-server, in a POST body) to
obtain a session token; the session token is what appears in URLs
and logs.

### Token design: HMAC-signed opaque blob

Use an HMAC-SHA256 signed token rather than JWT. Rationale:

- **Simpler.** No JWT library dependency. The token is
  `base64url(payload || hmac(payload))` where payload is
  `purpose + expiry + random-nonce`.
- **No header/claims parsing attack surface.** JWT `alg: none` and
  key-confusion attacks are eliminated by construction.
- **Opaque to the client.** The SPA treats it as an opaque string —
  no client-side decoding needed.
- **Server-side validation only.** The server checks HMAC, expiry,
  and purpose. No distributed verification needed (single-process
  architecture).

### Signing key lifecycle

- A 32-byte HMAC signing key is generated at first startup and
  persisted in the registry database (new table `signing_keys`).
- Only one key is active at a time. Rotation creates a new key and
  marks the old one inactive.
- Validation checks the active key only — rotating the key
  immediately invalidates all outstanding tokens.
- CLI command: `fox-control sec rotate-sse-key` rotates the key.

### Token endpoint

New endpoint: `POST /api/auth/session`

- **Auth:** `Authorization: Bearer <admin_secret>` (existing admin
  auth middleware).
- **Request body:** `{"purpose": "sse"}` (extensible for future
  token types).
- **Response:** `{"token": "<opaque>", "expires_at": "ISO-8601",
  "purpose": "sse"}`
- **Token lifetime:** 10 minutes (configurable via
  `control.session_token_ttl` in TOML config).

### SSE endpoint changes

`GET /api/events/stream` accepts the session token via:

1. **Cookie `fox_sse_token`** (preferred) — set by the SPA after
   obtaining the token. Same-origin, so no CORS issues.
2. **Query parameter `?token=`** (fallback) — for non-browser
   clients that can't set cookies.

Validation:

- Parse the token, verify HMAC against active signing key.
- Check `purpose == "sse"` — reject tokens with other purposes.
- Check expiry — reject expired tokens.
- **Reject the admin secret as the SSE token.** Even if someone
  manually passes the admin secret in the `?token=` parameter,
  it must be rejected. This is a defense-in-depth measure: the
  admin secret is never a valid SSE token. Implemented by checking
  that the token parses as a valid signed blob before accepting it.

On invalid token: respond with 401 and close the connection
immediately.

### SPA changes

1. After admin login, `POST /api/auth/session` with
   `{"purpose": "sse"}` to obtain a session token.
2. Store the token in `state.sseToken` (memory only, not
   `sessionStorage`).
3. Set a cookie `fox_sse_token=<token>; Path=/api/events;
   SameSite=Strict; Secure` (Secure only when on HTTPS).
4. Open `EventSource("/api/events/stream")` — the cookie is sent
   automatically.
5. Set a refresh timer at `TTL - 2 minutes` (default: 8 minutes for
   10-minute tokens). On refresh, POST again, update cookie, no SSE
   reconnection needed — the existing connection stays open; only
   new connections use the new cookie.
6. On 401 from SSE `onerror`: re-fetch the session token. If the
   admin secret itself is invalid (the user rotated it), fall
   through to `logout()`.

### Logging discipline

- Token generation: log token ID (first 8 chars of the nonce) and
  purpose. Never log the full token.
- Token validation failure: log reason (expired, bad signature,
  wrong purpose) and remote address. Never log the token bytes.
- Test: a test captures stdout/stderr during a 401 SSE attempt and
  asserts no token bytes appear in any log line.

## Consequences

### Positive

- Admin secret never appears in any URL. Log exposure is no longer
  an admin compromise.
- Session tokens are short-lived (10 min) — a leaked log entry has
  a narrow exploitation window.
- Token rotation invalidates all active sessions immediately.
- The `purpose` field enables future short-lived tokens for other
  use cases without design changes.

### Negative

- SPA complexity increases: token lifecycle management, cookie
  setting, refresh timer.
- Operators using curl for SSE debugging need a two-step flow:
  first POST for a token, then connect to SSE.
- An additional database table (`signing_keys`) is added.

### Cookie vs. URL-token decision

Cookie-based is preferred because:

- The token never appears in server/proxy logs.
- `SameSite=Strict` prevents CSRF.
- The browser manages cookie lifecycle automatically.

The URL `?token=` fallback exists for programmatic clients (curl,
monitoring tools) that can't set cookies. These clients are expected
to handle short-lived tokens — the trade-off is that the session
token (not the admin secret) appears in their logs, with a 10-minute
window.

### Compatibility

- **Panel SPA:** must be updated to use the new token flow.
  Old SPA versions (v1.0.0) will get 401 on SSE after the server
  upgrade — the admin secret in `?token=` is explicitly rejected.
  This is intentional: the security fix must not be bypassable by
  an old client.
- **CLI tooling:** not affected. CLI does not use SSE.
- **API clients using `Authorization: Bearer`:** not affected. All
  non-SSE endpoints continue to use admin secret in the header.
