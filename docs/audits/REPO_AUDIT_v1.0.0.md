# Repository Structure Audit — v1.0.0

Audit date: 2026-06-08

## Scope

Repository conventions, CI/CD configuration, OSS hygiene, and codebase
health at the v1.0.0 GA milestone.

---

## OSS standard files

| File | Status | Notes |
|------|--------|-------|
| `CODE_OF_CONDUCT.md` | Present | Contributor Covenant v2.1 |
| `CODEOWNERS` | Present | `@roadhero` as default reviewer |
| `.editorconfig` | Present | Go, YAML, JSON, Markdown |
| `CONTRIBUTING.md` | Present | Short version; `docs/contributing.md` has more |
| `LICENSE` | Present | Apache 2.0 |
| `NOTICE` | Present | Apache 2.0 Section 4(d) attribution |
| `THIRD-PARTY-LICENSES` | Present | 40 dependencies, all compatible |
| `GOVERNANCE.md` | **Missing** | Recommended for multi-contributor projects |
| `MAINTAINERS.md` | **Missing** | Recommended for contributor onboarding |
| `.gitattributes` | **Missing** | Recommended for consistent line endings |

---

## .gitignore audit

Current `.gitignore` covers: binaries, test artifacts, IDE files, vendor directory.

**Missing patterns (security-relevant):**

| Pattern | Risk |
|---------|------|
| `.env` / `.env.*` | Secrets in environment files |
| `*.key` / `*.pem` | Cryptographic material |
| `*.p12` / `*.pfx` / `*.jks` | Certificate stores |
| `credentials*` / `secrets*` | Credential files |
| `docker-compose.override.yml` | Local overrides with secrets |

---

## GitHub configuration

### Issue and PR templates

- `bug_report.md` — structured with environment fields (OS, Go, Docker, Fox Fleet version)
- `feature_request.md` — use case, proposed solution, alternatives
- `PULL_REQUEST_TEMPLATE.md` — enforces build/test/lint pass and ticket reference

### Workflows (3)

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `ci.yml` | Push/PR to main | Lint, test (race, shuffle, 45% coverage), multi-OS build, conformance |
| `release.yml` | Tag `v*.*.*` | Verify, build (5 platforms), release, .deb, Homebrew, container, signing |
| `docs.yml` | Push to main (docs/) | mkdocs-material build + GitHub Pages deploy |

---

## Makefile

6 targets: `build`, `test`, `lint`, `clean`, `conformance`, `bench`.

All targets functional. Build injects version/commit/date via ldflags.

---

## Codebase health

- **Go version:** 1.25.0 (`go.mod`)
- **Module path:** `github.com/fox-in-the-box-ai/fox-fleet`
- **Go source files:** 82
- **TODO/FIXME/HACK comments:** 0
- **Key dependencies:** cobra, docker, yaml.v3, toml, modernc.org/sqlite

---

## Directory structure

```
cmd/fox-control/       CLI entry point, config parsing, cobra subcommands
internal/
  config/              Config injection (instance data dirs, tools.json)
  events/              In-memory event log
  provisioner/         Provisioning orchestrator
  registry/            SQLite instance registry
plugins/
  plugin.go            DeploymentPlugin interface
  docker/              Docker plugin (7 operations)
panel/
  api/                 Dashboard HTTP API, health poller, source listing
  spa/                 Embedded SPA (instances + sources tabs)
conformance/
  runtime/             Runtime conformance suite (16 checks)
  plugin/              Plugin conformance suite (8 checks)
rollout/               Rolling update orchestration
data-plane/            Data plane server, Qdrant, ingestion, query API
skillsets/             Skillset manifest spec, YAML parser + validator
deploy/                Deployment configs (Compose, Helm, systemd, Caddy, Homebrew)
docs/                  Documentation site (mkdocs-material)
```

---

## Action items

| # | Severity | Item | Fix |
|---|----------|------|-----|
| 1 | High | `.gitignore` missing secret patterns | Add `.env*`, `*.key`, `*.pem`, credential patterns |
| 2 | Medium | `.gitattributes` missing | Add with `*.go text eol=lf`, binary patterns |
| 3 | Low | `GOVERNANCE.md` missing | Create minimal governance doc |
| 4 | Low | `MAINTAINERS.md` missing | Create maintainers list |
| 5 | Info | `coverage.out` and `fox-control` binary in repo root | Add to `.gitignore` if not already covered |

Items 1-2 tracked under REPO-AUDIT-02. Items 3-4 tracked under DOC-FINAL-04.
