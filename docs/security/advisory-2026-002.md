# Security Advisory 2026-002: Data Plane Query Endpoints Unauthenticated

**Severity:** Medium
**CVE:** N/A (pre-disclosure)
**Affected versions:** v0.2.0-alpha through v1.0.0
**Fixed in:** v1.0.1
**Issue:** [#100](https://github.com/fox-in-the-box-ai/fox-fleet/issues/100)
**ADR:** [0013-dataplane-instance-auth](../architecture/adr/0013-dataplane-instance-auth.md)

## Summary

The data plane query endpoints `POST /v1/query` and `GET /v1/sources` were publicly accessible without authentication. Any client with network access to the data plane could query the knowledge base and enumerate configured sources.

## Impact

An attacker with network access to the data plane port could read all indexed knowledge base content and list all configured data sources. The data plane admin endpoints (`/v1/admin/*`) were not affected — they were already behind admin auth.

## Mitigation

Upgrade to v1.0.1. Query endpoints now require authentication via per-instance query tokens or the admin secret.

### Upgrade steps

1. Update `fox-control` to v1.0.1
2. Restart the service — existing instances are automatically backfilled with query tokens in the registry
3. For each existing instance, run `fox-control sec rotate-query-token --instance <id>` to inject the token into the instance's config files, then restart the instance

New instances provisioned after the upgrade automatically receive query tokens.

### Authentication methods

The `requireQueryAuth` middleware accepts tokens via:
- `Authorization: Bearer <token>` header
- `X-Fox-Auth: <token>` header

Valid tokens are:
- The admin secret (same as used for admin endpoints)
- A per-instance query token (generated at provision time)

### CLI commands

```bash
# Rotate a query token for an existing instance
fox-control sec rotate-query-token --instance my-instance

# The instance must be restarted for the new token to take effect
```

## Timeline

- 2026-06-08: Issue reported (#100)
- 2026-06-08: ADR published (#104)
- 2026-06-08: Fix implemented and merged (#106)
- 2026-06-08: v1.0.1 released
