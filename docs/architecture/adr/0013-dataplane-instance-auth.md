# ADR 0013: Per-instance query tokens for the data plane

**Status:** Accepted  
**Date:** 2026-06-08  
**Issue:** [#100](https://github.com/fox-in-the-box-ai/fox-fleet/issues/100)

## Context

The data plane query endpoint `POST /v1/query` is unauthenticated.
Any process that can reach port 9091 can query the knowledge base
without credentials:

```
curl -s http://localhost:9091/v1/query \
  -d '{"query":"confidential roadmap"}'
```

The admin routes (`/v1/admin/*`) are correctly gated behind
`requireAdmin`, which checks `Authorization: Bearer <admin_secret>`
with constant-time comparison. But the query route was left public
under the assumption that only Fox instances would call it — an
assumption that doesn't hold once the data plane port is reachable
from the host network or from other containers on the same Docker
network.

The config injection layer (`internal/config/inject.go`) already
generates an `X-Fox-Auth` header in the `knowledge_query` tool
manifest and injects `FOX_PLANE_AUTH_SECRET` into each instance's
`hermes.env`. The data plane ignores this header on the query path.
All instances share the same credential (`admin_secret`), so a
compromised instance could escalate to full admin access on the
data plane.

## Decision

Add per-instance query tokens. Each instance authenticates to the
data plane with a unique, randomly generated token that grants query
access only. The admin secret retains full access (admin + query)
for operator tooling and the panel proxy.

### Token design: random opaque bearer token

Each token is 32 bytes of `crypto/rand`, encoded as unpadded
base64url (43 characters). No structure, no signing, no expiry —
validated by lookup against the registry.

Rationale for stateful lookup over signed tokens:

- **Single-process architecture.** The data plane and registry run
  in the same `fox-control` process. A database lookup is a local
  function call, not a network round-trip.
- **Instant revocation.** Deleting or rotating a token takes effect
  on the next request. Signed tokens with expiry would allow a
  revoked token to remain valid until it expires.
- **Simpler.** No signing key lifecycle, no HMAC verification, no
  clock-skew issues. The registry is already the source of truth
  for instance metadata.

### Registry schema change

Add a column to the `instances` table:

```sql
ALTER TABLE instances ADD COLUMN query_token TEXT NOT NULL DEFAULT ''
```

The migration runs in the existing `migrate()` function using the
same `ALTER TABLE ... ADD COLUMN` pattern already used for
`skillset_name` and `principal_role` (idempotent — SQLite ignores
the statement if the column exists).

On first startup after upgrade, existing instances will have
`query_token = ''`. The startup migration backfill (see below)
generates tokens for these rows.

### Token lifecycle

**Generation:** `crypto/rand` → 32 bytes → `base64url.RawEncoding`.
Generated at:

1. **Provision time** — `provisioner.Provision()` generates the
   token before calling `registry.Create()`.
2. **Startup migration backfill** — after schema migration,
   `registry.Open()` scans for rows with empty `query_token` and
   fills them. This handles the v1.0.0 → v1.0.1 upgrade path.
3. **Rotation** — `fox-control sec rotate-query-token --instance <id>`
   generates a new token, updates the registry, re-injects config
   into the instance's data directory, and restarts the container
   to pick up the new `hermes.env`.

**Storage:** the token is stored in the `query_token` column of the
`instances` table, alongside the instance ID.

**Injection:** the token replaces the admin secret as the data plane
credential in two config injection outputs:

- `hermes.env`: `FOX_DATA_PLANE_TOKEN=<token>` (new variable name,
  replaces `FOX_PLANE_AUTH_SECRET` for data plane auth).
- `tools.json`: the `knowledge_query` tool's `auth.env` field
  changes from `FOX_PLANE_AUTH_SECRET` to `FOX_DATA_PLANE_TOKEN`.

`FOX_PLANE_AUTH_SECRET` continues to be injected — it serves the
Fox instance's own `check_auth` gate for incoming requests from the
panel. Its role is instance-level auth, not data-plane auth. The
two credentials are now distinct.

### Data plane auth changes

`POST /v1/query` gets a new middleware `requireQueryAuth` that
accepts two credential types:

1. **Admin secret** — `Authorization: Bearer <admin_secret>`.
   Validated by constant-time comparison against the configured
   admin secret. Used by the panel proxy (`POST /api/query`
   → `POST /v1/query`) and operator curl.

2. **Instance query token** — `Authorization: Bearer <token>`.
   Validated by lookup: the data plane calls a `TokenValidator`
   interface to check whether the token matches any instance's
   `query_token`.

The two types are checked in order: admin secret first (O(1)
constant-time compare), then instance token lookup (O(n) scan, but
n is bounded by `max_instances` which defaults to 10). If neither
matches, respond 401.

```go
type QueryTokenValidator interface {
    ValidQueryToken(token string) bool
}
```

The `registry.Registry` implements this interface:

```go
func (r *Registry) ValidQueryToken(token string) bool {
    var count int
    err := r.db.QueryRow(
        `SELECT COUNT(*) FROM instances WHERE query_token = ?`,
        token,
    ).Scan(&count)
    return err == nil && count > 0
}
```

The data plane `server.New()` accepts a `QueryTokenValidator` as
a new parameter. When nil (data plane not connected to a registry),
query auth falls back to admin-secret-only.

### Panel proxy changes

The panel's `handleQuery` proxy (`panel/api/query_handler.go`) must
forward authentication to the data plane. Currently it creates a
bare `POST` with no auth header. After this change, it forwards the
admin secret:

```go
proxyReq.Header.Set("Authorization", "Bearer "+s.adminSecret)
```

This is safe: the panel already authenticated the operator via
`requireAuth` before reaching `handleQuery`. The admin secret is
an in-process string, not read from the incoming request.

### Config injection changes

In `internal/config/inject.go`:

**`renderHermesEnv`:** add `FOX_DATA_PLANE_TOKEN` to the env map
using the new `InjectParams.QueryToken` field. `FOX_PLANE_AUTH_SECRET`
remains (it serves instance-level auth).

**`renderToolsJSON`:** change the `knowledge_query` tool's
`auth.env` from `FOX_PLANE_AUTH_SECRET` to `FOX_DATA_PLANE_TOKEN`.
The `auth.header` stays `X-Fox-Auth` (the Fox runtime reads this
header name from the tool manifest and sends it on outbound tool
calls). The data plane's `requireQueryAuth` reads
`Authorization: Bearer` — the Fox runtime translates `auth.header`
+ `auth.env` into this standard form.

Wait — this needs clarification. Looking at the current tool
manifest:

```json
"auth": {"header": "X-Fox-Auth", "env": "FOX_PLANE_AUTH_SECRET"}
```

The Fox runtime reads `env` to get the credential value and sends
it in the header named by `header`. The data plane's `requireAdmin`
checks `Authorization: Bearer`. These are different headers.

Examining the current state: `requireAdmin` reads
`r.Header.Get("Authorization")` and strips `Bearer `. But the tool
manifest tells the Fox runtime to send `X-Fox-Auth`. This means
the current auth is already non-functional on the query path — the
instance sends `X-Fox-Auth` but the data plane (if it checked)
would look at `Authorization`. Since `/v1/query` is currently
unauthenticated, this mismatch has been invisible.

**Resolution:** the new `requireQueryAuth` middleware checks both
headers:
- `Authorization: Bearer <token>` — standard form, used by the
  panel proxy, operator curl, and API clients.
- `X-Fox-Auth: <token>` — used by Fox instances via the tool
  manifest. Checked as a fallback when `Authorization` is absent.

This avoids changing the tool manifest's `header` field, which
would require coordinating with the Fox runtime's tool-call
implementation.

### CLI command

New subcommand: `fox-control sec rotate-query-token --instance <id>`

1. Generate a new 32-byte random token.
2. Update `query_token` in the registry.
3. Re-inject config files into the instance's data directory.
4. Restart the instance container (via `plugin.Configure()`).
5. Log: `"query token rotated" instance=<id> token_prefix=<first 8 chars>`.

### Logging discipline

- Token generation: log instance ID and token prefix (first 8
  chars). Never log the full token.
- Token validation failure: log reason ("no Authorization or
  X-Fox-Auth header", "invalid query token") and remote address.
  Never log the token bytes.
- Migration backfill: log count of instances that received new
  tokens.

## Consequences

### Positive

- The query endpoint is no longer publicly accessible. Every
  request must present a valid credential.
- Per-instance tokens mean a compromised instance cannot
  impersonate another or escalate to admin access.
- Token rotation is per-instance — rotating one instance's token
  does not affect others.
- The panel proxy and operator tooling continue to work via the
  admin secret path.
- Backward compatibility: the migration auto-generates tokens
  for existing instances. No manual operator action required.

### Negative

- The registry gains a column and a lookup method. The
  `ValidQueryToken` query runs on every data plane request from
  instances (bounded by `max_instances`, typically small).
- Operators using curl to query the data plane now need
  `Authorization: Bearer <admin_secret>` — previously no auth
  was needed.
- An additional environment variable (`FOX_DATA_PLANE_TOKEN`) is
  injected into instances. The old `FOX_PLANE_AUTH_SECRET` remains
  for its original purpose (instance auth).

### Migration path

| Scenario | Behavior |
|----------|----------|
| Fresh v1.0.1 install | Tokens generated at provision time. No action needed. |
| v1.0.0 → v1.0.1 upgrade | `registry.Open()` detects empty `query_token` rows and backfills them. Config re-injection happens on next instance restart or configure call. |
| v1.0.1 → v1.0.0 rollback | SQLite ignores the unknown `query_token` column. Instances have `FOX_DATA_PLANE_TOKEN` in their env, which the v1.0.0 data plane ignores (query endpoint returns to unauthenticated). No breakage, but the security fix is lost. |

### Compatibility

- **Panel SPA:** not affected. The panel's query playground goes
  through `POST /api/query` on the panel server, which proxies
  with admin auth. No client-side changes.
- **CLI tooling:** not affected (CLI does not query the data plane).
- **Fox instances:** config re-injection updates `hermes.env` and
  `tools.json`. Instances pick up the new credential on next
  restart. Between upgrade and restart, instances send the old
  `FOX_PLANE_AUTH_SECRET` value (which is the admin secret) — this
  still works because `requireQueryAuth` accepts the admin secret.
- **External API clients:** any client directly hitting
  `POST /v1/query` must now include `Authorization: Bearer <secret>`.
  This is a breaking change for unauthenticated callers, which is
  the intended security fix.
