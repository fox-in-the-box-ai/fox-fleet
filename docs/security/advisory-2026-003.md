# Security Advisory 2026-003: v1.1.0 Security Hardening (SEC-01 through SEC-11)

**Severity:** Medium (aggregate; individual items range Low to Medium)
**CVE:** N/A (internal audit findings)
**Affected versions:** v1.0.0 through v1.0.1
**Fixed in:** v1.1.0

## Summary

An internal security audit identified 11 hardening opportunities across the management plane, data plane, and embedded SPA. None were actively exploited. All are fixed in v1.1.0.

## Findings

### SEC-01: Secret file permissions (Low)

`hermes.env` and `tools.json` were written with 0644 (world-readable). These files contain instance credentials and should be owner-only.

**Fix:** Secret files now written with 0600; non-secret config files remain 0644.

### SEC-02: SSRF in REST ingestion connector (Medium)

The REST connector followed user-supplied URLs without validating the resolved IP address. An attacker controlling a source config could probe internal services via the data plane.

**Fix:** HTTP transport uses a custom `net.Dialer` (`internal/safedialer`) that blocks private (RFC 1918), loopback, link-local, CGNAT (100.64/10), and unspecified addresses at the TCP dial level.

### SEC-03: Path traversal in file connector (Medium)

The file ingestion connector accepted any filesystem path. A source config with `path: /etc/shadow` or a symlink to sensitive files could exfiltrate data through the ingestion pipeline.

**Fix:** The file connector resolves symlinks via `filepath.EvalSymlinks`, then enforces a `filepath.Separator`-based prefix check against the configured `AllowedFileDir`.

### SEC-04: Missing HTTP write timeout (Low)

The panel HTTP server had no `WriteTimeout`, allowing slow-read attacks to hold connections indefinitely.

**Fix:** `WriteTimeout: 30s` added. SSE handler extends the deadline per-flush via `http.ResponseController` to avoid killing long-lived event streams.

### SEC-05: Unrestricted instance environment keys (Medium)

User-supplied `InstanceConfig.Env` could override reserved keys like `FOX_PLANE_AUTH_SECRET`, `PATH`, or `LD_PRELOAD`, potentially escalating privileges inside the container.

**Fix:** Env keys validated against a blocklist of reserved names. Blocked keys are rejected at provision time.

### SEC-06: Weak admin secret accepted (Low)

No minimum length on `auth.admin_secret` allowed trivially short or empty (bypassed by other checks) secrets.

**Fix:** Minimum 16-character length enforced at config validation. `fox-control generate-secret` command added for secure key generation.

### SEC-07: Instance ID format not validated on all endpoints (Low)

`handleDetail` and `handleDestroy` did not validate instance ID format, accepting arbitrary strings that could cause unexpected filesystem paths or log injection.

**Fix:** Instance ID format validation applied consistently across all handler entry points.

### SEC-08: Tag-only image references (Low)

Mutable image tags (`image: "ghcr.io/fox/runtime:stable"`) allow supply-chain substitution if the registry is compromised.

**Fix:** Startup logs a warning when the configured image uses a tag without a pinned digest (`@sha256:...`).

### SEC-09: Missing security headers on SPA (Low)

The embedded panel SPA served responses without standard security headers, leaving it open to clickjacking and MIME-sniffing attacks.

**Fix:** `Content-Security-Policy`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, and `Referrer-Policy: strict-origin-when-cross-origin` added to all SPA responses.

### SEC-10: YAML injection via fmt.Fprintf (Low)

`config.yaml` rendered with `fmt.Fprintf` template strings. Special characters in user-supplied values could produce malformed YAML or inject additional keys.

**Fix:** Rendering switched to `yaml.v3 Marshal`, which properly escapes all values.

### SEC-11: Unescaped Qdrant collection names (Low)

Collection names passed directly into URL path segments. Names containing `/`, `?`, or `%` could cause incorrect routing or path traversal in the Qdrant REST API.

**Fix:** `url.PathEscape()` applied to collection names in all Qdrant client URL constructions.

## Mitigation

Upgrade to v1.1.0. No configuration changes are required unless:

1. Your `admin_secret` is shorter than 16 characters — you must lengthen it (use `fox-control generate-secret`)
2. You use the file ingestion connector — set `data_plane.allowed_file_dir` to restrict accessible paths

## Timeline

- 2026-06-07: Internal audit completed (50-ticket backlog panel deliberation)
- 2026-06-08: All 11 fixes implemented and tested
- 2026-06-08: v1.1.0 released
