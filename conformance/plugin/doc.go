// Package plugin implements the 8 plugin conformance checks that validate
// a DeploymentPlugin implementation: provision, health, configure, rollout,
// rollback, destroy, idempotency, and error handling.
//
// Tickets: CONF-02 (Plugin conformance test suite)
// Spec: fox-in-the-box/docs/architecture/ENTERPRISE_ARCHITECTURE.md §6.4 (8 checks)
//       fox-in-the-box/docs/architecture/FLEET_BASE_V01_SPEC.md §1 (provisioning timeout = 120s)
// Milestone: v0.1
package plugin
