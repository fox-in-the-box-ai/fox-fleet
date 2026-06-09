# Documentation Completeness Audit — v1.4.2

Audit date: 2026-06-09

## Scope

Full inventory, accuracy verification, and link integrity check of all
documentation in the fox-fleet repository at the v1.4.2 milestone.

---

## Inventory

54 markdown files, 9,053 total lines. 14 doc.go package docs. 1 OpenAPI spec.

### User-facing documentation (31 files)

| File | Lines | Category | Audited | Status |
|------|-------|----------|---------|--------|
| `README.md` | 291 | Root documentation | Yes | Fixed — added 4 missing internal/ packages, dataplane conformance suite, version bump |
| `SECURITY.md` | 50 | Security policy | Yes | Fixed — expanded supported versions, corrected session token model |
| `CONTRIBUTING.md` | 47 | Contributing guide | Yes | Clean |
| `CHANGELOG.md` | 214 | Release notes | Yes | Clean |
| `CODE_OF_CONDUCT.md` | 9 | Community standards | Yes | Clean |
| `docs/index.md` | 78 | Documentation landing | Yes | Clean |
| `docs/getting-started.md` | 104 | Installation guide | Yes | Clean |
| `docs/DEPLOYMENT.md` | 476 | Deployment guide | Yes | Fixed in LOOSE-02 — broken README link, stale config refs |
| `docs/configuration.md` | 207 | Configuration reference | Yes | Fixed — added missing [tls], [[webhooks]], [rate_limit], [auto_restart] sections |
| `docs/WALKTHROUGH.md` | 224 | Operator tutorial | Yes | Clean |
| `docs/LIMITATIONS.md` | 106 | Architecture boundaries | Yes | Clean |
| `docs/operator/handbook.md` | 837 | Operator handbook | Yes | Fixed — 18+ corrections: Qdrant version, config field names, CLI commands, auth model |
| `docs/install/linux.md` | 251 | Linux install | Yes | Fixed — 16 version refs updated |
| `docs/install/macos.md` | 197 | macOS install | Yes | Fixed — 11 version refs updated |
| `docs/install/windows.md` | 147 | Windows install | Yes | Fixed — 3 version refs updated |
| `docs/install/kubernetes.md` | 161 | Kubernetes install | Yes | Fixed — 3 version refs updated |
| `docs/quickstart/linux.md` | 185 | Linux quickstart | Yes | Clean |
| `docs/quickstart/macos.md` | 164 | macOS quickstart | Yes | Clean |
| `docs/quickstart/windows.md` | 165 | Windows quickstart | Yes | Clean |
| `docs/quickstart/kubernetes.md` | 166 | Kubernetes quickstart | Yes | Clean |
| `docs/security/signing.md` | 134 | Binary verification | Yes | Fixed — 16 version refs updated |
| `docs/security/advisory-2026-001.md` | 49 | Security advisory | Yes | Clean |
| `docs/security/advisory-2026-002.md` | 54 | Security advisory | Yes | Clean |
| `docs/security/advisory-2026-003.md` | 91 | Security advisory | Yes | Clean |
| `docs/changelog.md` | 7 | mkdocs changelog proxy | Yes | Clean |
| `docs/contributing.md` | 46 | mkdocs contributing proxy | Yes | Clean |
| `docs/README.md` | 5 | mkdocs readme proxy | Yes | Clean |
| `examples/minimal/README.md` | 41 | Example docs | Yes | Clean |
| `examples/production/README.md` | 68 | Example docs | Yes | Clean |
| `examples/air-gapped/README.md` | 54 | Example docs | Yes | Clean |
| `docs/release/runbook.md` | 212 | Release runbook | Yes | Fixed — asset/sig counts, CGO_ENABLED value |

### Engineering documentation (14 files)

| File | Lines | Category | Audited | Status |
|------|-------|----------|---------|--------|
| `docs/DEVELOPER.md` | 424 | Developer guide | Yes | Fixed — added dataplane suite, fixed Qdrant version |
| `docs/TICKET_MAP.md` | 51 | Ticket tracking | Yes | Fixed — CONF-01 count 16→24, added DP-09 |
| `docs/REPO_SETUP.md` | 42 | Repository setup | Yes | Clean |
| `docs/architecture/APACHE_BACKLOG_v1.md` | 960 | Feature backlog | Yes | Clean |
| `docs/architecture/APACHE_ROADMAP_v1.x.md` | 216 | Release roadmap | Yes | Clean |
| `docs/architecture/BACKLOG.md` | 58 | Backlog index | Yes | Clean |
| `docs/architecture/PANEL_DELIBERATION_v1_backlog.md` | 171 | Panel design notes | Yes | Clean |
| `docs/architecture/PRODUCTS.md` | 127 | Product definitions | Yes | Clean |
| `docs/architecture/adr/0012-sse-session-tokens.md` | 172 | ADR: SSE auth | Yes | Clean |
| `docs/architecture/adr/0013-dataplane-instance-auth.md` | 304 | ADR: Dataplane auth | Yes | Clean |
| `docs/architecture/adr/0014-apache-enterprise-boundary-v1.1.md` | 50 | ADR: Enterprise boundary | Yes | Clean |
| `docs/architecture/adr/0015-binary-integration-tests.md` | 37 | ADR: Integration tests | Yes | Rewritten in LOOSE-03 |
| `docs/architecture/adr/0016-docker-sdk-daemon-vulns-accepted.md` | 31 | ADR: Docker vulns | Yes | New in LOOSE-01 |

### Audit history (7 files)

| File | Lines | Category | Audited | Status |
|------|-------|----------|---------|--------|
| `docs/audits/DOCS_AUDIT_v1.0.0.md` | 91 | v1.0.0 doc audit | Yes | Historical — superseded by this audit |
| `docs/audits/ENDUSER_DOCS_v1.0.0.md` | 187 | v1.0.0 end-user docs | Yes | Historical |
| `docs/audits/GRAND_AUDIT_v1.4.0.md` | 400 | v1.4.0 grand audit | Yes | Historical |
| `docs/audits/INSTALL_COVERAGE_v1.0.0.md` | 445 | v1.0.0 install coverage | Yes | Historical |
| `docs/audits/REPO_AUDIT_v1.0.0.md` | 118 | v1.0.0 repo audit | Yes | Historical |
| `docs/audits/SECURITY_AUDIT_v1.0.0.md` | 138 | v1.0.0 security audit | Yes | Historical |
| `docs/audits/V1.0.0_FINALIZATION.md` | 146 | v1.0.0 finalization | Yes | Historical |

### GitHub templates (3 files)

| File | Lines | Category | Audited | Status |
|------|-------|----------|---------|--------|
| `.github/PULL_REQUEST_TEMPLATE.md` | 12 | PR template | Yes | Updated in LOOSE-03 — ADR-0015 checklist item |
| `.github/ISSUE_TEMPLATE/bug_report.md` | 22 | Issue template | Yes | Clean |
| `.github/ISSUE_TEMPLATE/feature_request.md` | 11 | Issue template | Yes | Clean |

### Non-markdown documentation (15 files)

| File | Category | Audited | Status |
|------|----------|---------|--------|
| `openapi.yaml` | API spec (28 endpoints) | Yes | Fixed — version 1.4.1→1.4.2 |
| `Makefile` | Build reference | Yes | Fixed — test flags aligned with CI |
| `deploy/helm/fox-control/Chart.yaml` | Helm chart | Yes | Fixed — version/appVersion 1.4.2 |
| 14 × `doc.go` | Package documentation | Yes | Fixed conformance/runtime/doc.go check count; see Known Gaps |

---

## Accuracy Verification Method

Every documentation claim was verified against the actual binary, source code,
and configuration parser — not proofread for grammar. Five parallel audit agents
each covered a non-overlapping document group:

1. **Operator docs** — handbook.md, configuration.md, DEPLOYMENT.md
2. **Install + quickstart** — 8 install/quickstart docs, signing.md, Chart.yaml
3. **OpenAPI + developer** — openapi.yaml, DEVELOPER.md, TICKET_MAP.md, Makefile, README.md
4. **Architecture + security** — SECURITY.md, 5 ADRs, conformance doc.go, runbook.md
5. **Examples + Helm** — 3 example READMEs, Chart.yaml, cross-cutting config refs

Each agent was instructed to read actual Go source, config structs, CLI help
output, and Makefile targets to verify claims. Findings were fixed in-place.

---

## Link Integrity (AUDIT-DOC-04)

### Internal markdown links

- **Total files scanned:** 54
- **Total internal links found:** 86
- **Valid:** 85
- **Broken:** 0 (1 context-dependent — `.github/PULL_REQUEST_TEMPLATE.md` uses
  a repo-root-relative path that resolves correctly on GitHub where the template
  is rendered)

### doc.go cross-repo spec references

12 doc.go files reference 5 spec documents from the parent `fox-in-the-box`
project that do not exist in this repository:

| Missing Document | Referenced by |
|------------------|---------------|
| `ENTERPRISE_ARCHITECTURE.md` | conformance/runtime, conformance/plugin, plugins/, plugins/docker/ |
| `DEMO_TIER.md` | internal/config, internal/provisioner, internal/registry, panel/api, panel/spa |
| `DATA_PLANE.md` | data-plane/, data-plane/qdrant/ |
| `FLEET_BASE_V01_SPEC.md` | rollout/ |
| `SKILLSETS.md` | skillsets/ |

These are cross-repo references to architecture specs from an earlier project
phase. They are pre-existing (present since v1.0.0) and not a v1.4.2 regression.
Tracked for future resolution — either migrate the specs into this repo or
update the references to point to current equivalents.

---

## Cross-Link Check (AUDIT-DOC-05)

The end-user application (`fox-in-the-box/hermes-webui`) does not cross-link to
fox-fleet documentation. No broken cross-repo references in either direction
for user-facing docs. The doc.go spec references above are the only cross-repo
gap.

---

## Fixes Applied

### Commits in this audit pass

| Commit | Scope | Files |
|--------|-------|-------|
| `3f5d445` | LOOSE-01: Docker SDK upgrade, govulncheck filter | go.mod, go.sum, ci.yml, ADR-0016 |
| `882bd90` | LOOSE-02: Broken README link in DEPLOYMENT.md | docs/DEPLOYMENT.md |
| `477ce46` | LOOSE-03: Ratify ADR-0015, update PR template | ADR-0015, PR template |
| `fbf7296` | Version refs 1.4.1/1.0.0→1.4.2 | install/*.md, signing.md, Chart.yaml, kubernetes.md |
| `09ca295` | Operator docs accuracy | handbook.md, configuration.md |
| `4b1521c` | Developer/security/architecture accuracy | openapi.yaml, Makefile, README.md, SECURITY.md, conformance/runtime/doc.go, DEVELOPER.md, TICKET_MAP.md, runbook.md |

### Summary of drift found and corrected

- **Version references:** 49 stale version numbers across install, signing, Helm, and OpenAPI docs
- **Config field names:** 4 wrong field names in operator handbook (cooldown vs cooldown_seconds, etc.)
- **Missing config sections:** 4 TOML sections absent from configuration.md ([tls], [[webhooks]], [rate_limit], [auto_restart])
- **Missing CLI subcommands:** 5 commands undocumented in handbook (sec rotate-sse-key, sec rotate-query-token, backup, restore, diagnostics)
- **Wrong counts:** Conformance check count (16→24), release asset count (21→23), sig/pem count (5→7)
- **Wrong defaults/values:** CGO_ENABLED (1→0), log output target (stdout→stderr), Qdrant image version (v1.13.2/v1.13.3→v1.14.1)
- **False claims:** "no incremental ingestion" (incorrect), wrong webhook HMAC signing description, wrong data plane auth model
- **Missing packages in README:** 4 internal/ subdirectories (events/, output/, safedialer/, sessiontoken/)
- **Missing suite in DEVELOPER.md:** Dataplane conformance suite undocumented

---

## Conclusion

All 54 markdown files, 14 doc.go files, the OpenAPI spec, Makefile, and Helm
chart have been audited against the actual codebase. Every finding has been
corrected and committed. Internal link integrity is clean (85/86, with the
1 context-dependent link functioning correctly on GitHub). The only open
documentation gap is the 5 cross-repo spec references in doc.go files, which
are pre-existing and tracked for future resolution.
