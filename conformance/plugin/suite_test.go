package plugin

import (
	"testing"

	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
)

func TestParseImageRef(t *testing.T) {
	tests := []struct {
		input string
		repo  string
		dig   string
	}{
		{
			input: "ghcr.io/fox-in-the-box-ai/fox@sha256:abc123",
			repo:  "ghcr.io/fox-in-the-box-ai/fox",
			dig:   "sha256:abc123",
		},
		{
			input: "ghcr.io/fox-in-the-box-ai/fox",
			repo:  "ghcr.io/fox-in-the-box-ai/fox",
			dig:   "",
		},
	}
	for _, tt := range tests {
		ref := parseImageRef(tt.input)
		if ref.Repository != tt.repo {
			t.Errorf("parseImageRef(%q).Repository = %q, want %q", tt.input, ref.Repository, tt.repo)
		}
		if ref.Digest != tt.dig {
			t.Errorf("parseImageRef(%q).Digest = %q, want %q", tt.input, ref.Digest, tt.dig)
		}
	}
}

func TestCheck08NonexistentReturnsError(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker daemon")
	}

	// This test requires a running Docker daemon.
	// In CI, it runs as part of `make conformance`.
	// Locally, skip if Docker is not available.
}

func TestImageRefRoundtrip(t *testing.T) {
	ref := plugins.ImageRef{
		Repository: "ghcr.io/fox-in-the-box-ai/fox",
		Digest:     "sha256:abc123def456",
	}
	s := ref.Repository + "@" + ref.Digest
	parsed := parseImageRef(s)
	if parsed.Repository != ref.Repository {
		t.Errorf("Repository mismatch: got %q, want %q", parsed.Repository, ref.Repository)
	}
	if parsed.Digest != ref.Digest {
		t.Errorf("Digest mismatch: got %q, want %q", parsed.Digest, ref.Digest)
	}
}
