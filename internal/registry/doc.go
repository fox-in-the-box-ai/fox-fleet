// Package registry implements the SQLite-backed instance registry: CRUD
// operations for provisioned Fox instances with port uniqueness enforcement
// and WAL mode for concurrent access.
//
// Tickets: CTRL-01 (Instance registry)
// Spec: fox-in-the-box/docs/architecture/DEMO_TIER.md §2.4 (table schema)
// Milestone: v0.1
package registry
