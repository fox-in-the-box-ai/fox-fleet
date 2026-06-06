// Package runtime implements the 16 runtime conformance checks that validate
// a Fox instance satisfies the instance contract: boot invariant, auth paths,
// contract endpoints, SSE events, and lifecycle.
//
// Tickets: CONF-01 (Runtime conformance test suite)
// Spec: fox-in-the-box/docs/architecture/ENTERPRISE_ARCHITECTURE.md section 6.3 (16 checks)
// Milestone: v0.1
package runtime
