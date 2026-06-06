// Package spa holds the embedded single-page dashboard: vanilla HTML + CSS + JS,
// no framework, no build step. Served by the Go binary via embed.
//
// Tickets: PANEL-02 (Dashboard SPA)
// Spec: fox-in-the-box/docs/architecture/DEMO_TIER.md §3.4 (technology: vanilla JS, ~200 lines)
//       fox-in-the-box/docs/architecture/FLEET_BASE_V01_SPEC.md §1 (auto-refresh = 5s)
// Milestone: v0.1
package spa
