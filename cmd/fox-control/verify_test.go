package main

import "testing"

func TestExtractTag(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "linux amd64",
			filename: "fox-control-v1.0.0-linux-amd64.tar.gz",
			want:     "v1.0.0",
		},
		{
			name:     "alpha tag",
			filename: "fox-control-v0.3.0-alpha-darwin-arm64.tar.gz",
			want:     "v0.3.0-alpha",
		},
		{
			name:     "rc tag",
			filename: "fox-control-v1.0.0-rc.1-linux-amd64.tar.gz",
			want:     "v1.0.0-rc.1",
		},
		{
			name:     "windows",
			filename: "fox-control-v2.1.0-windows-amd64.tar.gz",
			want:     "v2.1.0",
		},
		{
			name:     "with path prefix",
			filename: "./artifacts/fox-control-v1.0.0-linux-arm64.tar.gz",
			want:     "v1.0.0",
		},
		{
			name:     "beta with path",
			filename: "/tmp/downloads/fox-control-v1.0.0-beta.2-darwin-amd64.tar.gz",
			want:     "v1.0.0-beta.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTag(tt.filename)
			if got != tt.want {
				t.Errorf("extractTag(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}
