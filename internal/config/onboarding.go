package config

import (
	"os"
	"path/filepath"
)

// MarkOnboardingComplete writes onboarding.json so managed instances skip the
// /setup <-> /login redirect loop when HERMES_WEBUI_PASSWORD is set.
// Fox bootstraps with completed=false; call this after the instance is healthy.
func MarkOnboardingComplete(dataDir string) error {
	dir := filepath.Join(dataDir, "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "onboarding.json")
	return os.WriteFile(path, []byte(`{"completed":true}`+"\n"), 0o644)
}
