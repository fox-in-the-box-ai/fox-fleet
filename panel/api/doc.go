// Package api implements the Fleet dashboard REST API: list, detail,
// provision, and destroy Fox instances. All endpoints require
// Authorization: Bearer {admin_secret}.
//
// Tickets: PANEL-01 (Dashboard API)
// Spec: fox-in-the-box/docs/architecture/DEMO_TIER.md §3.4 (dashboard views)
//       fox-in-the-box/docs/architecture/FLEET_BASE_V01_SPEC.md §1 (health_poll_interval = 15s)
// Milestone: v0.1
package api
