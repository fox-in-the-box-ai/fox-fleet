# ADR 0016: Docker SDK Daemon-Side Vulnerabilities Accepted

**Status:** Accepted
**Date:** 2026-06-09

## Context

govulncheck reports two vulnerabilities in `github.com/docker/docker`:

- **GO-2026-4887** — Moby AuthZ plugin bypass when provided oversized request bodies
- **GO-2026-4883** — Moby off-by-one error in plugin privilege validation

Both are server-side Docker daemon vulnerabilities affecting AuthZ plugin handling and plugin privilege validation within the Docker Engine. Neither vulnerability has an upstream fix available (`Fixed in: N/A` across all published versions through v28.5.2).

Fox Fleet uses the Docker client SDK (`github.com/docker/docker/client`) to communicate with a local Docker daemon over a Unix socket. The vulnerable code paths are in the daemon's plugin authorization middleware, which Fox Fleet never imports, instantiates, or exercises. govulncheck flags them because the client SDK and daemon code share the same Go module (`github.com/docker/docker`), and `init()` chains in shared type packages create symbol-level reachability.

## Decision

Accept the risk for GO-2026-4887 and GO-2026-4883. The CI govulncheck step excludes these two IDs and fails on any other vulnerability.

Rationale:

1. **Not reachable in our binary.** The vulnerable code paths are daemon-side AuthZ plugin and privilege validation. Fox Fleet's binary is a client — it never runs Docker's authorization middleware.
2. **No upstream fix available.** The Docker project has not tagged a release that resolves either CVE. Downgrading or switching SDK versions does not help.
3. **Operator mitigation exists.** The daemon itself should be patched by upgrading Docker Engine on the host. Fox Fleet documents Docker Engine ≥ 24.0 as a prerequisite; operators who run a current Docker Engine are protected at the daemon layer.

## Consequences

- CI govulncheck runs in JSON mode with a jq filter that excludes GO-2026-4887 and GO-2026-4883. Any new vulnerability in any dependency still fails the build.
- When Docker releases a fix, remove the exclusion and bump the dependency. Track via: `govulncheck ./...` returning clean without the filter.
- The Docker SDK was bumped from v27.5.1 to v28.5.2 as part of this decision to stay on the latest client API.
