// Package docker implements DeploymentPlugin for Docker, extracted from
// the Electron app's docker-manager.js. Provisions Fox containers with
// per-instance naming, port mapping, and health polling.
//
// Tickets: PLUG-02 (Docker plugin implementing DeploymentPlugin)
// Spec: fox-in-the-box/docs/architecture/ENTERPRISE_ARCHITECTURE.md section 2.2 (Docker plugin)
//       fox-in-the-box/docs/architecture/DEMO_TIER.md section 2.2 (extraction plan)
// Milestone: v0.1
package docker
