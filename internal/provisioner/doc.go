// Package provisioner implements the per-instance provisioning loop:
// allocate port, create data directory, write config files, call the
// deployment plugin, and poll health until ready or timeout.
//
// Tickets: CTRL-03 (provisioning orchestrator)
// Spec: fox-in-the-box/docs/architecture/DEMO_TIER.md §3.2 (instance lifecycle)
//       fox-in-the-box/docs/architecture/FLEET_BASE_V01_SPEC.md §1 (health timeout = 120s)
//       fox-in-the-box/docs/architecture/FLEET_BASE_V01_SPEC.md §2.4 (risk: serialize provisioning)
// Milestone: v0.1
package provisioner
