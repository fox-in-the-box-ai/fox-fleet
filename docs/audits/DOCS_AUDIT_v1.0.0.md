# Documentation Audit — v1.0.0

Audit date: 2026-06-08

## Scope

Complete inventory and accuracy check of all documentation in the fox-fleet
repository at the v1.0.0 GA milestone.

---

## Inventory

20 markdown files, 1,913 total lines.

| File | Lines | Category |
|------|-------|----------|
| `docs/DEPLOYMENT.md` | 452 | Deployment guide |
| `README.md` | 283 | Root documentation |
| `docs/WALKTHROUGH.md` | 224 | Tutorial |
| `docs/configuration.md` | 141 | Configuration reference |
| `docs/security/signing.md` | 134 | Security |
| `docs/LIMITATIONS.md` | 124 | Architecture boundaries |
| `docs/getting-started.md` | 104 | Installation guide |
| `CHANGELOG.md` | 97 | Release notes |
| `docs/index.md` | 78 | Documentation landing |
| `docs/TICKET_MAP.md` | 50 | Engineering reference |
| `docs/contributing.md` | 46 | Contributing guide |
| `SECURITY.md` | 44 | Security policy |
| `docs/REPO_SETUP.md` | 42 | Repository setup |
| `CONTRIBUTING.md` | 29 | Contributing guide (root) |
| `.github/ISSUE_TEMPLATE/bug_report.md` | 22 | Issue template |
| `.github/PULL_REQUEST_TEMPLATE.md` | 11 | PR template |
| `.github/ISSUE_TEMPLATE/feature_request.md` | 11 | Issue template |
| `CODE_OF_CONDUCT.md` | 9 | Community guidelines |
| `docs/changelog.md` | 7 | Changelog redirect |
| `docs/README.md` | 5 | Docs folder index |

---

## Findings

### Critical

1. **SECURITY.md supported versions table is stale.** Lists `v0.1.x (upcoming)`
   when v1.0.0 is GA. Must read `v1.0.x | Yes` and `< v1.0.0 | No (alpha)`.

### High

2. **LIMITATIONS.md planned improvements table** — shows shipped features
   labeled `Shipped (v0.3.0-alpha)`. Confusing now that v1.0.0 is the GA
   release. Update to `Shipped (v1.0.0)` or `Shipped (v0.3.0-alpha, included in v1.0.0)`.

3. **CHANGELOG.md line 95** — references "v0.1.0 stable" which was never
   released as a standalone version. Should reference v1.0.0.

### Low

4. **LIMITATIONS.md line 98** — references `v0.3.0` for i18n. Accurate
   historically but could clarify this is now in v1.0.0.

5. **README.md roadmap table** — v0.1/v0.2/v0.3 rows use `0.x.0-alpha`
   labels, which are historically correct. The table is clear as-is (shows
   both the milestone version and "Shipped" status) but could benefit from a
   note that all alpha milestones are subsumed by v1.0.0.

### Compliant (no issues)

- All badges accurate (CI, License, Go version).
- Quickstart commands verified against current code.
- Go 1.25+ requirement consistently stated across all files.
- `docs/security/signing.md` references v1.0.0 correctly.
- `docs/DEPLOYMENT.md` updated to v1.0.0 examples.
- `docs/getting-started.md` installation methods accurate.
- `docs/WALKTHROUGH.md` tutorial current.
- `docs/configuration.md` matches current TOML schema.
- Zero broken internal links across all 20 files.

---

## Action items

| # | Severity | File | Fix |
|---|----------|------|-----|
| 1 | Critical | `SECURITY.md` | Update supported versions table to v1.0.x |
| 2 | High | `docs/LIMITATIONS.md` | Update planned improvements Shipped labels |
| 3 | High | `CHANGELOG.md` | Fix "v0.1.0 stable" reference |
| 4 | Low | `docs/LIMITATIONS.md` | Clarify i18n version reference |
| 5 | Low | `README.md` | Consider note on alpha → GA subsumption |

All items tracked under DOC-FINAL-02.
