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

func TestValidateRequiresThumbnailCacheDir(t *testing.T) {
	cfg := Default()
	cfg.Roots = []RootConfig{{ID: "media", Path: t.TempDir()}}
	cfg.Thumbnail.CacheDir = ""

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "thumbnail.cache_dir") {
		t.Fatalf("expected thumbnail cache validation error, got %v", err)
	}
}
