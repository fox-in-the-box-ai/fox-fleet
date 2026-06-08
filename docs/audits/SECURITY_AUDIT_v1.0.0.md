# Security Audit — v1.0.0

Audit date: 2026-06-08

## Scope

Security review of all Go source files (~30 files), the embedded SPA, CI
workflows, dependency supply chain, and deployment configurations at the
v1.0.0 GA milestone. Threat model: authenticated admin on a trusted
network (per SECURITY.md), with secondary attention to what an
unauthenticated network observer or co-tenant host user could exploit.

---

## Findings

### High

**H1. `hermes.env` written world-readable (0644) — contains auth secrets**
- `internal/config/inject.go:179` — `os.WriteFile` uses `0o644` for all
  generated config files including `hermes.env`, which contains
  `FOX_PLANE_AUTH_SECRET` and `HERMES_WEBUI_PASSWORD` in plaintext.
- Impact: co-tenant host user reads secrets, authenticates to panel API.
- Fix: write `hermes.env` with mode `0o600`.
- Tracked: v1.0.1 patch.

**H2. SSE endpoint transmits admin secret as URL query parameter**
- `panel/api/auth.go:26-28` — SSE path reads token from `r.URL.Query()`.
- `panel/spa/static/index.html:1136` — JS passes secret in EventSource URL.
- Impact: admin secret appears in server/proxy access logs, browser history.
- Fix: exchange a short-lived session token via POST; use that in the SSE URL.
- Tracked: filed as issue (requires design work, not a bug fix).

**H3. REST ingestion connector is an SSRF proxy**
- `data-plane/ingestion/rest/connector.go:43-49` — accepts arbitrary URL
  from source config, issues HTTP GET to it.
- Impact: authenticated admin can probe internal services (cloud metadata,
  Qdrant, localhost endpoints).
- Fix: validate URL scheme + block RFC-1918/link-local/loopback ranges.
- Tracked: v1.0.1 patch.

**H4. File ingestion connector reads arbitrary filesystem paths**
- `data-plane/ingestion/file/connector.go:36-49` — `config.path` from user
  input passed directly to `os.ReadFile`.
- Impact: authenticated admin reads any file the process can access.
  Combined with unauthenticated `/v1/query` (M2), ingested data becomes
  publicly queryable.
- Fix: restrict paths to a configurable allowed directory.
- Tracked: v1.0.1 patch.

### Medium

**M1. No `WriteTimeout` on control-plane HTTP server**
- `cmd/fox-control/main.go:158-162` — `ReadHeaderTimeout` set but no
  `WriteTimeout`. Slow-read clients can pin goroutines.
- Tracked: v1.0.1 patch.

**M2. Data plane `/v1/query` endpoint is unauthenticated**
- `data-plane/server/server.go:63` — registered outside `requireAdmin`.
- Impact: if data plane binds to `0.0.0.0` (as docker-compose config does),
  anyone on the network can query the knowledge base.
- Tracked: filed as issue (design decision — query endpoint is intentionally
  open for instances, but needs instance-scoped auth).

**M3. Docker-compose healthcheck hits authenticated endpoint**
- `deploy/docker-compose/docker-compose.yml:21-22` — healthcheck calls
  `/api/instances` which requires auth. Will always return 401.
- Fix: change to `/healthz`.
- Tracked: v1.0.1 patch.

**M4. Container env injection — no key validation**
- `plugins/docker/plugin.go:58-60` — `InstanceConfig.Env` keys not
  validated. Admin could override security-sensitive env vars.
- Tracked: v1.0.1 patch.

**M5. Admin secret has no minimum length enforcement**
- `cmd/fox-control/config.go:134-138` — only validates `!= ""`.
- Tracked: v1.0.1 patch.

### Low / Informational

**I1.** `handleDetail` and `handleDestroy` don't validate instance ID
against `validInstanceID` regex (defense-in-depth; not exploitable due
to parameterized SQL and Docker API string handling).

**I2.** Docker image reference allows tag-only (no digest). Rollout
enforces digest, but `serve`/`provision` accept `:latest`.

**I3.** No security headers on the embedded SPA (CSP, X-Frame-Options,
X-Content-Type-Options).

**I4.** `config.yaml` rendered via `fmt.Fprintf` — YAML-special characters
in values could produce malformed output. Should use `yaml.v3` marshaling.

**I5.** Qdrant collection name not URL-escaped in API path construction.

---

## Positive findings

- Constant-time auth comparison (`crypto/subtle.ConstantTimeCompare`)
  in both panel and data-plane auth.
- Request body size limits (`http.MaxBytesReader`) on all mutating endpoints.
- Instance ID validated against `^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`.
- All SQLite queries use parameterized `?` placeholders.
- XSS prevention in SPA via `esc()` text-node escaping.
- Secrets never logged in auth failure path.
- No shell execution from user input.
- `ReadHeaderTimeout` set on both servers.
- Container ports bound to `127.0.0.1` by default.
- Environment variable overrides for secrets (avoid config file secrets).
- Secret-bearing files covered by `.gitignore`.

---

## Dependency supply chain

6 direct dependencies, ~30 indirect. All licenses Apache-2.0/MIT/BSD-3
compatible. Full inventory in `THIRD-PARTY-LICENSES`.

Recommend adding `govulncheck` to CI for continuous vulnerability scanning.

---

## Action items

| # | Severity | Fix | Target |
|---|----------|-----|--------|
| H1 | High | `hermes.env` file permissions → 0o600 | v1.0.1 |
| H2 | High | SSE session token (design work) | Issue filed |
| H3 | High | SSRF protection on REST connector | v1.0.1 |
| H4 | High | File path restriction on file connector | v1.0.1 |
| M1 | Medium | Add `WriteTimeout` to HTTP server | v1.0.1 |
| M2 | Medium | Query endpoint auth (design decision) | Issue filed |
| M3 | Medium | Fix docker-compose healthcheck URL | v1.0.1 |
| M4 | Medium | Env key validation in Docker plugin | v1.0.1 |
| M5 | Medium | Admin secret minimum length | v1.0.1 |
| I1-I5 | Low | Defense-in-depth improvements | v1.x backlog |
