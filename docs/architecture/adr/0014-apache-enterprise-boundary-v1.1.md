# ADR 0014: Apache vs Enterprise Boundary Decisions (v1.x Backlog)

**Status:** Accepted
**Date:** 2026-06-08
**Deciders:** 6-agent panel (ARCH-1, ARCH-2, ARCH-3, SWE-1, SWE-2, SWE-3) — all decisions unanimous (6/6)

## Context

While producing the post-v1.0.1 Apache Base forward backlog, the panel identified three features that sit on the Apache/Enterprise boundary. Each has a plausible open-source argument ("proves pluggability," "essential for adoption," "expected by operators") and a plausible commercial argument ("ongoing maintenance burden," "enterprise-scale complexity," "commercial differentiation"). These decisions needed explicit resolution to prevent scope creep into the Apache backlog.

The open-core model for Fox Fleet draws the line at:
- **Apache Base (free):** Single-host Docker management plane, embedded panel, data plane with file/REST ingestion, single bearer-token auth.
- **Enterprise (commercial):** Multi-host orchestration, RBAC, audit trail, advanced integrations, managed deployment targets.

## Decision 1: Open WebUI Adapter → Enterprise

**Decision:** The Open WebUI adapter remains Enterprise scope. It is NOT ticketed in the Apache backlog.

**Rationale:**
- The `DeploymentPlugin` interface is already proven by the Docker plugin and the conformance suite (CONF-01). The extension point exists and works.
- An Open WebUI adapter is a runtime-specific integration requiring ongoing maintenance against a third-party API surface that Fox Fleet does not control.
- The Hermes adapter is the reference implementation. Additional runtime adapters are commercial differentiation — they expand the addressable market rather than hardening the core.
- The plugin development guide (DOC-02) enables third-party contributors to build adapters without them being in the Apache codebase.

## Decision 2: Kubernetes Deployment Plugin → Enterprise

**Decision:** The K8s deployment plugin remains Enterprise scope. It is NOT ticketed in the Apache backlog.

**Rationale:**
- K8s per-instance pod management requires service mesh integration, PVC provisioning, RBAC, and cross-node coordination — all enterprise-scale concerns.
- The existing Helm chart covers deploying `fox-control` itself on K8s. Per-instance pods are a fundamentally different architecture from single-host Docker containers.
- The `DeploymentPlugin` interface is the Apache extensibility point; the K8s implementation is the commercial product.
- **Boundary principle:** single-host Docker = free, multi-host orchestration = paid.

## Decision 3: Webhook Forwarding Scope → Split

**Decision:** Simple POST-on-event with HMAC-SHA256 signing is Apache (INT-01). Retry with exponential backoff, dead-letter queue, payload transforms, conditional routing, and multi-endpoint fan-out are Enterprise.

**Rationale:**
- The boundary is drawn at delivery semantics: best-effort fire-and-forget with HMAC signing is the Apache ceiling.
- This covers ~80% of operator use cases (Slack notifications, PagerDuty alerts, simple HTTP integrations) with approximately 50 lines of Go and zero additional dependencies.
- Once retry semantics are added, the feature requires delivery tracking, persistent queue state, and a management UI — substantial infrastructure that justifies commercial licensing.
- Size estimate confirms the boundary: Apache webhook is S (1–2 days) vs Enterprise pipeline is L (1–2 weeks).

## Consequences

- Apache backlog contains 50 tickets without any Enterprise-scope features. Scope pressure on these three features is explicitly resolved.
- Contributors building runtime adapters (Open WebUI, LM Studio, etc.) use the `DeploymentPlugin` interface and the development guide (DOC-02). Their adapters can live in separate repos under any license.
- Webhook consumers needing at-least-once delivery, transforms, or routing upgrade to Enterprise.
- These decisions should be revisited if the open-source community demonstrates sustained demand for any of the three features as Apache scope (e.g., multiple external contributors building and maintaining K8s support).
