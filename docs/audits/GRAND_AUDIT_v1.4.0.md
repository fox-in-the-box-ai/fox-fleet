# Apache Fleet OS — Grand Adversarial Audit v1.4.0

**Audit date:** 2026-06-09
**Audit target:** Fox Fleet v1.4.0 (commit 3ff06cd on main)
**Published release:** v1.4.0 (GitHub Release, marked Latest)

---

## TL;DR

**Verdict: patch-then-ship.** The management plane (provisioning, monitoring, updates, CLI, panel) is production-quality and matches the PRD. The data plane — the entire knowledge/ingestion/query feature — is dead code at runtime: the HTTP server package exists and is tested, but is never imported or started by the binary. Seven CRITICAL findings must be patched in v1.4.1 before operator outreach. Seven HIGH findings should be fixed before the Apertu demo. Neither class requires architectural changes — all are wiring, config, or documentation fixes.

**50/50 backlog tickets shipped.** Code exists for every promised feature. The gap is not missing implementations but missing wiring: three features (data plane server, auto-restart, rate limit config) are fully coded but disconnected from the binary's startup path or config struct.

---

## Methodology

Four independent reviewers audited v1.4.0 from different perspectives:

| Reviewer | Perspective | Source of truth |
|----------|-------------|-----------------|
| **REVIEW-PRD** | Promise vs shipped | PRODUCTS.md, ADRs, CHANGELOG, roadmaps |
| **REVIEW-CODE** | Code vs spec | Every Go package, config field, API route, CLI command |
| **REVIEW-ARTIFACT** | Release artifacts | GitHub Release, Dockerfile, Compose, Helm, systemd, Homebrew, APT |
| **REVIEW-UX** | Operator experience | SPA panel, CLI help, documentation, onboarding flow |

**Round 1:** Each reviewer produced findings independently.
**Round 2:** Cross-review — findings confirmed, amended, or disputed across reviewers.
**Round 3:** Synthesis — deduplicated and assigned consensus severity.
**Round 4:** Recommendations — fix path for each CRITICAL and HIGH finding.

Severity definitions:
- **CRITICAL** — blocks operator outreach (must fix in v1.4.1)
- **HIGH** — blocks Apertu demo (should fix in v1.4.1)
- **MEDIUM** — post-outreach patch (v1.4.1 or v1.5)
- **LOW** — nits, future improvements

---

## Findings summary

| Severity | Count |
|----------|-------|
| CRITICAL | 7 |
| HIGH | 7 |
| MEDIUM | 11 |
| LOW | 13 |
| **Total** | **38** |

---

## CRITICAL findings

### C-1. Data plane HTTP server is never started

**Caught by:** REVIEW-CODE, verified by REVIEW-PRD
**Evidence:** `grep -rn "data-plane/server" cmd/ internal/ panel/ plugins/` returns zero results. `cmd/fox-control/main.go` imports only `data-plane/source` (the SQLite source registry), never `data-plane/server` (the HTTP server).

The `data-plane/server` package is fully implemented: 9 API routes, auth middleware, 17 test functions. But `cmd/fox-control/main.go` never imports it, never constructs a `server.Server`, and never calls `ListenAndServe`. When `data_plane.enabled = true`, the binary opens the source database and constructs a proxy URL — but the HTTP server that should listen at that URL is never started. All panel `POST /api/query` requests proxy to a dead port.

**Impact:** The entire knowledge feature — ingestion, vector search, query API — is non-functional at runtime despite being fully implemented and advertised.

**Recommendation:** Patch in v1.4.1. Import `data-plane/server`, construct and start it in `newServeCmd` when `data_plane.enabled = true`.

---

### C-2. Auto-restart is a phantom feature

**Caught by:** REVIEW-PRD, REVIEW-CODE, REVIEW-UX (3 independent reviewers)
**Evidence:**
- `panel/api/poller.go:20-35` — `AutoRestartConfig` struct with `Enabled`, `Threshold`, `Cooldown` fields
- `panel/api/poller.go:130-165` — `checkAutoRestart()` fully implemented
- `cmd/fox-control/config.go` — no `AutoRestart` field on `Config` struct
- `panel/api/server.go` — no `AutoRestart` field on `Deps` struct
- `cmd/fox-control/main.go` — never sets any auto-restart config
- `docs/operator/handbook.md:~580-593` — documents `[auto_restart]` TOML section as operational

The feature is coded in the poller, documented in the handbook, but electrically disconnected. `HealthPoller.autoRestart` is always its zero value (`Enabled: false`). No TOML field, no Deps field, no wiring in main. Operators who follow the handbook to enable auto-restart will set config keys that are silently ignored.

**Impact:** Phantom feature — documented and implemented but cannot be activated.

**Recommendation:** Patch in v1.4.1. Add `AutoRestart AutoRestartConfig` to `Config` struct with TOML tags, add to `Deps`, wire in `newServeCmd`.

---

### C-3. Dockerfile runs as root

**Caught by:** REVIEW-ARTIFACT
**Evidence:** `grep -n "USER\|adduser\|addgroup" Dockerfile` returns no results. No `USER` directive exists.

The container image runs as UID 0 by default. The systemd install creates a dedicated `fox-control` user, and the Helm chart sets `allowPrivilegeEscalation: false` + `capabilities.drop: [ALL]`, but Docker and Compose users get root. This contradicts the project's security hardening claims.

**Impact:** Root-equivalent container process for all non-K8s container deployments.

**Recommendation:** Patch in v1.4.1. Add `RUN addgroup -S fox && adduser -S -G fox fox` and `USER fox` before `ENTRYPOINT`. Ensure data directory is owned by the new user.

---

### C-4. CGO_ENABLED mismatch between release workflow and Dockerfile

**Caught by:** REVIEW-ARTIFACT
**Evidence:**
- `Dockerfile:10` — `RUN CGO_ENABLED=1 go build ...`
- `.github/workflows/release.yml:99` — `CGO_ENABLED: "0"`

The binary artifacts (tarballs, deb packages) are built with `CGO_ENABLED=0`. The container image is built with `CGO_ENABLED=1` plus `apk add gcc musl-dev`. The project uses `modernc.org/sqlite` (CGO-free), making CGO=1 unnecessary and potentially introducing different behavior between artifacts.

**Impact:** Container image and binary releases are built under different compiler modes.

**Recommendation:** Patch in v1.4.1. Change Dockerfile to `ENV CGO_ENABLED=0`, remove `gcc musl-dev` from apk install.

---

### C-5. systemd MemoryDenyWriteExecute=true crashes Go on modern kernels

**Caught by:** REVIEW-ARTIFACT
**Evidence:** `deploy/systemd/fox-control.service:36` — `MemoryDenyWriteExecute=true`

Go's runtime allocates executable memory pages (for the GC write barrier and runtime internals). On Linux kernel 6.3+ with strict seccomp enforcement (Ubuntu 24.04, Debian 13), this causes SIGSYS. Older kernels may not enforce it, masking the issue until the operator upgrades their OS.

**Impact:** systemd deployments will crash on current LTS distributions.

**Recommendation:** Patch in v1.4.1. Remove `MemoryDenyWriteExecute=true` from the service unit.

---

### C-6. Operator handbook documents wrong CLI flags for backup and restore

**Caught by:** REVIEW-UX
**Evidence:**
- `docs/operator/handbook.md:396` — `fox-control backup --dest ...` (wrong)
- `docs/operator/handbook.md:410` — `fox-control restore --from ...` (wrong)
- `docs/operator/handbook.md:741` — troubleshooting references `--dest`
- `docs/operator/handbook.md:801-802` — CLI reference table uses `--dest` and `--from`
- `cmd/fox-control/ops.go:62` — actual flag is `--output`
- `cmd/fox-control/ops.go:117` — actual flag is `--input`

Every handbook reference to backup/restore uses non-existent flags. Operators following the documented procedure get `Error: unknown flag: --dest`.

**Impact:** Documented backup/restore procedures are broken.

**Recommendation:** Patch in v1.4.1. Replace `--dest` with `--output` and `--from` with `--input` throughout the handbook.

---

### C-7. Homebrew tap repository does not exist

**Caught by:** REVIEW-ARTIFACT
**Evidence:** `gh api repos/fox-in-the-box-ai/homebrew-fox-fleet` returns HTTP 404. The formula template (`deploy/homebrew/fox-control.rb.tmpl`) is correctly authored, and the release workflow has a `homebrew-tap` job, but the target repository doesn't exist.

`docs/install/macos.md` documents `brew tap fox-in-the-box-ai/fox-fleet && brew install fox-control` — this will fail for all macOS users.

**Impact:** Documented macOS install channel is broken.

**Recommendation:** Create `github.com/fox-in-the-box-ai/homebrew-fox-fleet` with a `Formula/` directory before outreach, or remove Homebrew from install docs until the repo exists.

---

## HIGH findings

### H-1. CSP blocks Google Fonts, degrading panel typography

**Caught by:** REVIEW-UX
**Evidence:**
- `panel/api/server.go:165` — CSP: `style-src 'self' 'unsafe-inline'` (no Google Fonts domains)
- `panel/spa/static/index.html:7-9` — loads Sora and Manrope from `fonts.googleapis.com`

When the browser enforces the CSP, both brand typefaces are blocked. The panel falls back to `system-ui, sans-serif`, losing the designed visual identity.

**Recommendation:** Patch in v1.4.1. Either add `https://fonts.googleapis.com` to `style-src` and `https://fonts.gstatic.com` to `font-src`, or self-host the fonts (preferred — eliminates external dependency and privacy concern).

---

### H-2. Rate limit config section is documented but inoperative

**Caught by:** REVIEW-PRD, REVIEW-CODE, REVIEW-UX (3 reviewers)
**Evidence:**
- `cmd/fox-control/config.go:86-89` — `RateLimitSection` struct exists but is not a field of `Config`
- `panel/api/server.go:40` — `Deps.ProvisionRateLimit` exists but is never set by `main.go`
- `docs/operator/handbook.md` — documents `[rate_limit]` TOML section with `requests_per_second` and `burst`

The effective rate limit is always the hardcoded default of 100 req/min. Operators who configure rate limits via TOML get no effect and no error.

**Recommendation:** Patch in v1.4.1. Add `RateLimit RateLimitSection` to `Config`, wire to `Deps.RateLimit` and `Deps.ProvisionRateLimit` in `newServeCmd`.

---

### H-3. Install docs hardcode stale v1.0.0 artifact versions

**Caught by:** REVIEW-ARTIFACT
**Evidence:** `grep -c "v1.0.0"` — linux.md (13 hits), macos.md (10 hits), windows.md (3 hits). Users copying documented commands will download non-existent v1.0.0 artifacts from GitHub Releases (404).

**Recommendation:** Patch in v1.4.1. Update all version references to v1.4.0. Consider using a `FOX_VERSION` variable pattern so future updates require a single change.

---

### H-4. Qdrant health never surfaces in /healthz

**Caught by:** REVIEW-CODE
**Evidence:** `panel/api/server.go` — `Deps.QdrantHealth` is an interface field that `main.go` never sets. The poller's `if p.qdrant != nil` check is always false. The `/healthz` endpoint never reports Qdrant status regardless of `qdrant.enabled`.

**Recommendation:** Patch in v1.4.1. Construct a `qdrant.Client` and set `Deps.QdrantHealth` when `cfg.Qdrant.Enabled` is true.

---

### H-5. ADR-0012 SSE `?token=` query parameter fallback not implemented

**Caught by:** REVIEW-PRD
**Evidence:** ADR-0012 specifies both cookie delivery and `?token=` query parameter fallback for non-browser clients. `panel/api/auth.go:48-53` — `extractSessionToken` reads only the `fox_sse_token` cookie. No query parameter fallback exists.

**Impact:** curl and programmatic SSE clients cannot authenticate.

**Recommendation:** Patch in v1.4.1. Add `r.URL.Query().Get("token")` fallback in `extractSessionToken` after cookie check.

---

### H-6. Sources empty state provides no actionable path

**Caught by:** REVIEW-UX
**Evidence:** Sources can only be created via the data plane admin API on port 9091 (a separate HTTP surface). The panel's empty state says "Add file or REST sources" but has no button, link, or reference to how this is done.

**Recommendation:** Patch in v1.4.1. Add actionable text: "Sources are managed via the data plane admin API. See the operator handbook for ingestion commands."

---

### H-7. Keyboard navigation broken for interactive table rows

**Caught by:** REVIEW-UX
**Evidence:** Instance cards, source rows, and skillset rows use `data-action` attributes and `cursor:pointer` but are plain `<div>` or `<tr>` elements with no `tabindex`, `role`, or keyboard event handler. Keyboard-only users cannot navigate to detail views.

**Recommendation:** Patch in v1.4.1 or v1.5. Add `tabindex="0"`, `role="link"`, and Enter/Space keyboard handlers to interactive rows.

---

## MEDIUM findings

| # | Finding | Source | Evidence |
|---|---------|--------|----------|
| M-1 | openapi.yaml license field says "Business Source License 1.1"; actual license is Apache 2.0 | REVIEW-PRD | `openapi.yaml:9` vs `LICENSE` |
| M-2 | openapi.yaml version stuck at 0.1.0 (should be 1.4.0) | REVIEW-PRD, REVIEW-CODE | `openapi.yaml:7` |
| M-3 | configuration.md missing `[tls]`, `[[webhooks]]`, `log_format`, `log_level`, `session_token_ttl_seconds`, `metrics_enabled` fields and 7 CLI subcommands | REVIEW-CODE, REVIEW-UX | `docs/configuration.md` |
| M-4 | docker.Plugin.Configure is a no-op stub — silently drops config changes | REVIEW-CODE | `plugins/docker/plugin.go:163-172` |
| M-5 | `sec rotate-query-token` does not restart container per ADR-0013 spec | REVIEW-PRD | `cmd/fox-control/main.go:535-590` |
| M-6 | `checkEmbedding` in diagnostics only checks URL non-empty, never pings the service | REVIEW-CODE | `cmd/fox-control/ops.go:260-262` |
| M-7 | `qdrant.Manager` (Docker lifecycle management) fully implemented but never instantiated | REVIEW-CODE | `data-plane/qdrant/qdrant.go` |
| M-8 | No HEALTHCHECK instruction in Dockerfile (only in Compose) | REVIEW-ARTIFACT | `Dockerfile` |
| M-9 | install.sh performs sha256 verification but not cosign signature verification | REVIEW-ARTIFACT | `install.sh` |
| M-10 | SECURITY.md supported-versions table shows only v1.0.x; should cover v1.x | REVIEW-UX | `SECURITY.md:22` |
| M-11 | Modal dialogs have no `role="dialog"`, `aria-modal`, `aria-labelledby`, or focus trap | REVIEW-UX | `panel/spa/static/index.html` |

---

## LOW findings

| # | Finding | Source |
|---|---------|--------|
| L-1 | `go 1.25.0` in go.mod does not correspond to a released Go version | REVIEW-CODE |
| L-2 | No favicon — browser tabs show generic icon | REVIEW-UX |
| L-3 | Browser back button navigates away from panel (no `pushState`) | REVIEW-UX |
| L-4 | `pointID` computed and discarded on every chunk upsert (dead assignment) | REVIEW-CODE |
| L-5 | `registry.GetQueryToken` exported but never called | REVIEW-CODE |
| L-6 | `ingestion.Plugin` interface defined but never implemented or referenced | REVIEW-CODE |
| L-7 | PERF-02 batch embedding only in file connector, not REST connector | REVIEW-PRD |
| L-8 | ADR-0010 and ADR-0011 referenced in audit brief but absent from repo | REVIEW-PRD |
| L-9 | INSTANCE_CONTRACT.md, ENTERPRISE_ARCHITECTURE.md, FLEET_BASE_ROADMAP.md absent | REVIEW-PRD |
| L-10 | Diagnostics `checkDisk` only verifies directory exists, not available space | REVIEW-PRD |
| L-11 | No systemd uninstall script (removal is inline in docs only) | REVIEW-ARTIFACT |
| L-12 | Quickstart step 2: `cp .env.example .env` followed by `cat > .env` overwrites the copy | REVIEW-UX |
| L-13 | `outline: none` on inputs without `:focus-visible` compensation (WCAG 2.4.7) | REVIEW-UX |

---

## PRD-shipping matrix (summary)

| Document | Claims audited | MATCH | DRIFT | MISSING |
|----------|---------------|-------|-------|---------|
| PRODUCTS.md Apache "Yes" items | 38 | 35 | 3 | 0 |
| CHANGELOG entries | ~80 | ~77 | 3 | 0 |
| ADR-0012 (SSE sessions) | 7 clauses | 6 | 1 | 0 |
| ADR-0013 (query tokens) | 8 clauses | 7 | 1 | 0 |
| ADR-0014 (Apache/Enterprise) | 4 clauses | 4 | 0 | 0 |
| Backlog (50 tickets) | 50 | 50 | 0 | 0 |

**DRIFT items:** auto-restart (documented, not wired), rate limit config (documented, not wired), SSE `?token=` fallback (specified in ADR, not coded), per-provision rate limit (claimed, not wired), rotate-query-token restart (specified in ADR, not executed).

**Enterprise boundary:** clean. No enterprise-only features found in Apache code. RBAC, SSO/OIDC, audit logs, K8s pod plugin, multi-host — all verified absent.

---

## Code-spec drift matrix (summary)

| Package | Purpose | Spec match | Tests | Drift |
|---------|---------|------------|-------|-------|
| `cmd/fox-control` | CLI + config | Partial | Yes | Dead RateLimitSection, data plane not started |
| `internal/registry` | SQLite instance store | OK | Yes | Dead GetQueryToken export |
| `internal/provisioner` | Instance lifecycle | OK | Yes (19 tests) | — |
| `internal/config` | Config injection | OK | Yes | — |
| `internal/events` | Event log + SSE | OK | Yes | — |
| `internal/sessiontoken` | HMAC tokens | OK | Yes | — |
| `panel/api` | Panel HTTP server | Partial | Yes (9 files) | Auto-restart + Qdrant health dead |
| `plugins/docker` | Docker plugin | Partial | Yes | Configure is no-op |
| `rollout` | Rolling update | OK | Yes | — |
| `skillsets` | YAML manifests | OK | Yes | — |
| `data-plane/server` | Data plane HTTP | PHANTOM | Yes (17 tests) | Never imported or started |
| `data-plane/source` | Source registry | OK | Yes | — |
| `data-plane/embedding` | Embedding client | OK | Yes | — |
| `data-plane/qdrant` | Qdrant client + mgr | Partial | Yes | Manager never instantiated |
| `conformance/*` | Test suites | OK | Yes | — |

---

## Release artifact verification (summary)

| Channel | Status | Blocking issue |
|---------|--------|----------------|
| GitHub Release | GREEN | — |
| Container image (Dockerfile) | RED | Runs as root (C-3), CGO mismatch (C-4) |
| Docker Compose | GREEN | — |
| Helm chart | GREEN | Minor: no pod-level runAsNonRoot |
| systemd | RED | MemoryDenyWriteExecute crashes Go (C-5) |
| Homebrew | RED | Tap repo doesn't exist (C-7) |
| APT/deb packages | GREEN | — |
| Install script | YELLOW | No cosign verification (M-9) |
| Install docs | RED | Stale v1.0.0 references (H-3) |
| Cosign signing | GREEN | — |
| SBOM | GREEN | — |

---

## Recommendations

### v1.4.1 patch backlog (CRITICAL + HIGH — before outreach)

| ID | Fix | Size | Blocking |
|----|-----|------|----------|
| FIX-01 | Import and start `data-plane/server` in main.go when `data_plane.enabled=true` | S | C-1 |
| FIX-02 | Wire `AutoRestartConfig` from Config → Deps → HealthPoller | S | C-2 |
| FIX-03 | Add non-root USER to Dockerfile | XS | C-3 |
| FIX-04 | Change Dockerfile to `CGO_ENABLED=0`, remove gcc/musl-dev | XS | C-4 |
| FIX-05 | Remove `MemoryDenyWriteExecute=true` from systemd unit | XS | C-5 |
| FIX-06 | Fix backup/restore flags in handbook (`--dest`→`--output`, `--from`→`--input`) | XS | C-6 |
| FIX-07 | Create `homebrew-fox-fleet` tap repo with Formula/ directory | XS | C-7 |
| FIX-08 | Fix CSP to allow Google Fonts or self-host the fonts | S | H-1 |
| FIX-09 | Wire `RateLimitSection` from Config → Deps | XS | H-2 |
| FIX-10 | Update all install docs from v1.0.0 to v1.4.0 | S | H-3 |
| FIX-11 | Wire `Deps.QdrantHealth` when Qdrant is enabled | XS | H-4 |
| FIX-12 | Add `?token=` query parameter fallback in `extractSessionToken` | XS | H-5 |
| FIX-13 | Add actionable text to sources empty state | XS | H-6 |
| FIX-14 | Add tabindex + keyboard handlers to interactive table rows | S | H-7 |

**Estimated total: ~2-3 days of implementation.**

### v1.5+ defer list

| Finding | Reason to defer |
|---------|-----------------|
| M-4 docker.Plugin.Configure stub | Needs design decision: live reconfig vs restart |
| M-7 qdrant.Manager dead code | Needs product decision: managed vs external Qdrant |
| L-3 Browser back button / pushState | SPA routing redesign |
| L-6 ingestion.Plugin interface cleanup | Refactor, no user impact |
| L-7 REST connector batch embedding | Performance optimization |

### Reframe the PRD/docs

| Finding | Action |
|---------|--------|
| M-1 openapi.yaml license | Change to "Apache License 2.0" |
| M-2 openapi.yaml version | Change to "1.4.0" |
| M-3 configuration.md gaps | Add missing sections |
| M-5 rotate-query-token restart | Document manual restart requirement |
| M-10 SECURITY.md version table | Update to v1.x |
| L-12 Quickstart step 2 | Remove redundant `cp .env.example .env` |

### Accept as known limitation

| Finding | Justification |
|---------|---------------|
| L-1 go 1.25.0 directive | Builds work; will self-resolve when Go 1.25 ships |
| L-8 Missing ADRs 0010-0011 | Decisions are reflected in code; ADR docs are nice-to-have |
| L-9 Missing spec docs | PRODUCTS.md + ADR-0014 provide adequate boundary definition |
| L-10 Diagnostics disk check | Directory existence is sufficient for v1.x |

---

## Bottom line

**Apache Fleet OS is not ready for operator outreach or the Apertu demo at v1.4.0.**

The management plane (provisioning, monitoring, updates, panel, CLI) is solid — production-quality code with good test coverage, clean architecture, and a professional SPA. The 50-ticket backlog is fully shipped. The enterprise boundary is clean.

But three categories of issues must be resolved first:

1. **Dead wiring** (C-1, C-2, H-2, H-4): The data plane server, auto-restart, rate limit config, and Qdrant health are fully implemented but disconnected. These are 4 small wiring fixes, not feature gaps.

2. **Artifact defects** (C-3, C-4, C-5, C-7): The Dockerfile runs as root with wrong CGO flags, systemd will crash on modern kernels, and the Homebrew tap doesn't exist. These are each one-line to few-line fixes.

3. **Documentation drift** (C-6, H-1, H-3): The handbook uses wrong CLI flags, the CSP breaks the panel's fonts, and install docs point at v1.0.0 artifacts. These are text/config changes.

**None of these require architectural changes.** The estimated fix effort for all 14 CRITICAL + HIGH items is 2-3 days. After v1.4.1 patches, the product is ready for outreach and demo.
