package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMarkOnboardingComplete(t *testing.T) {
	dir := t.TempDir()
	if err := MarkOnboardingComplete(dir); err != nil {
		t.Fatalf("MarkOnboardingComplete: %v", err)
	}
	path := filepath.Join(dir, "config", "onboarding.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read onboarding.json: %v", err)
	}
	want := "{\"completed\":true}\n"
	if string(data) != want {
		t.Errorf("onboarding.json = %q, want %q", data, want)
	}
}
