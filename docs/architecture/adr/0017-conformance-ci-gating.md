# ADR-0017: Conformance CI Gating

**Date:** 2026-06-09
**Status:** Accepted
**Deciders:** Dennis Vorobyov

## Context

The CI workflow includes a conformance job that builds a Docker image from the
repository (`fox-control:ci`) and runs the conformance suite against it.

Starting with the conformance suite's introduction in v1.3.0, this job has
never passed on CI. Investigation of the failing run
([run 27175224125](https://github.com/fox-in-the-box-ai/fox-fleet/actions/runs/27175224125/job/80222580253))
identified a three-layer protocol mismatch:

1. **Port mismatch.** The SUT runner
   (`conformance/runtime/sut/runner.go:18`) maps container port 8080.
   The `fox-control` binary defaults to port 9090
   (`cmd/fox-control/config.go:122`).

2. **Missing config file.** The Dockerfile CMD requires
   `/etc/fox-control/fox-control.toml` — the SUT never mounts one.
   `LoadConfig` (`cmd/fox-control/config.go:99`) fails immediately,
   causing the container to exit non-zero.

3. **Protocol mismatch.** The conformance suite tests the **Fox instance
   protocol** — endpoints `/health`, `/readyz`, `/version`,
   `/capabilities`, `/api/v1/auths/signup`, `/api/chat/completions`.
   The `fox-control` management plane serves a different API:
   `/healthz` and `/api/*` (`panel/api/server.go:151–156`).
   The management plane does not implement the Fox instance HTTP contract.

CI results from run 27175224125:
- Check 01 (Boot invariant): **PASS** — uses `BootInvariant` mode which
  skips health check; the container exiting non-zero is expected behavior
- Checks 02–24: **FAIL** — `health check timed out after 2m0s`
- Plugin conformance: **never ran** — the job step was skipped after
  runtime conformance failed

### Alternatives considered

| Alternative | Why not viable in v1.4.2 |
|-------------|--------------------------|
| Build a Fox instance image in CI (sibling container) | Requires the Fox runtime image, which is a separate repository (`fox-in-the-box`). Cross-repo image builds are a v1.5 scope item. |
| Self-hosted runner with a pre-pulled Fox image | Requires infrastructure changes (runner provisioning, image registry access). Not justified for a maintenance release. |
| Rootless DinD with Fox image | Same cross-repo image dependency. Also adds DinD complexity to the CI matrix. |
| Fix the SUT to test the management plane | The conformance suite's purpose is to verify Fox *instance* protocol compliance. Changing it to test the management plane would invalidate its design intent. |

## Decision

Gate the conformance CI job on `workflow_dispatch` only, so it does not run
on push or PR triggers. The job remains in the workflow and can be triggered
manually when a proper Fox instance image is available.

Changes made:
- Added `workflow_dispatch:` to CI workflow triggers
- Added `if: ${{ github.event_name == 'workflow_dispatch' }}` to the
  conformance job
- Updated README to say "conformance suite" (not "conformance in CI")

## Consequences

### Positive

- CI pipeline no longer fails on a structurally unfixable test
- Conformance suite code is preserved — no deletion, no bit-rot
- Manual trigger allows ad-hoc conformance runs against real Fox images

### Negative

- Conformance is not automatically enforced. If nobody triggers it, regressions
  in the conformance suite itself go undetected.
- The `workflow_dispatch` trigger requires someone to remember to run it.

### Risk: silent atrophy

The primary risk is that `workflow_dispatch` with no scheduled trigger becomes
"effectively deleted." Two mitigations are in place:

1. **Release checklist item** — the release runbook
   (`docs/release/runbook.md`) includes a pre-release step to trigger
   conformance manually when a Fox instance image is available.
2. **GitHub issue** — a tracked issue for restoring automated conformance
   in v1.5 ensures the work is not forgotten.

## Restoration plan

**Target:** v1.5.0

To restore automated conformance in CI:

1. Publish a Fox instance image to a container registry accessible from
   GitHub Actions (e.g., `ghcr.io/fox-in-the-box-ai/fox-in-the-box:latest`)
2. Update the conformance CI job to pull this image instead of building
   `fox-control:ci`
3. Remove the `workflow_dispatch` gate — conformance runs on every push/PR
4. Close the tracking issue

The conformance suite code requires no changes — only the CI job's image
source needs updating.
