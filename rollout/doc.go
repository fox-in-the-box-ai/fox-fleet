// Package rollout orchestrates rolling image updates across a fleet of Fox
// instances: pull new digest, update one at a time, health-check between
// each, rollback on failure.
//
// Tickets: REL-01 (Fleet rollout orchestration)
// Spec: fox-in-the-box/docs/architecture/FLEET_BASE_V01_SPEC.md section 1 (rollout health timeout = 120s)
// Milestone: v0.1
package rollout
