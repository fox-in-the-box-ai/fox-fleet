// Package config handles fox-control.toml parsing and per-instance config
// injection: writes hermes.env, config.yaml, and settings.json to each
// instance's data directory before container start.
//
// Tickets: CTRL-02 (config injection), CTRL-04 (config file parsing)
// Spec: fox-in-the-box/docs/architecture/DEMO_TIER.md §3.1 (TOML format)
//       fox-in-the-box/docs/architecture/INSTANCE_CONTRACT.md §5 (injection files)
// Milestone: v0.1
package config
