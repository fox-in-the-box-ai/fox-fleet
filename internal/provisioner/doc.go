// Package provisioner implements the per-instance provisioning loop:
// allocate port, create data directory, write config files, call the
// deployment plugin, and poll health until ready or timeout.
//
// Tickets: CTRL-03 (provisioning orchestrator)
// Spec: fox-in-the-box/docs/architecture/DEMO_TIER.md section 3.2 (instance lifecycle)
// Milestone: v0.1
package provisioner
