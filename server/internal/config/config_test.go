package config

import (
	"strings"
	"testing"
)

func TestValidateRequiresConfiguredDefaultProfile(t *testing.T) {
	cfg := Default()
	cfg.Roots = []RootConfig{{ID: "media", Path: t.TempDir()}}
	cfg.Playback.DefaultProfile = "missing"

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "default_profile") {
		t.Fatalf("expected default profile validation error, got %v", err)
	}
}
