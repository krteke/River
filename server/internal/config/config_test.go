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

func TestValidateAllowsDirectProfileWithoutTranscodeSettings(t *testing.T) {
	cfg := Default()
	cfg.Roots = []RootConfig{{ID: "media", Path: t.TempDir()}}
	cfg.Playback.DefaultProfile = "original"
	cfg.Profiles = []ProfileConfig{{Name: "original", Direct: true}}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected direct profile to validate, got %v", err)
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

func TestValidateRejectsInvalidHardwareBackend(t *testing.T) {
	cfg := Default()
	cfg.Roots = []RootConfig{{ID: "media", Path: t.TempDir()}}
	cfg.Hardware.Enabled = true
	cfg.Hardware.Backend = "cuda"

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "hardware_transcode.backend") {
		t.Fatalf("expected hardware backend validation error, got %v", err)
	}
}

func TestValidateRejectsInvalidHardwareQuality(t *testing.T) {
	cfg := Default()
	cfg.Roots = []RootConfig{{ID: "media", Path: t.TempDir()}}
	cfg.Hardware.Enabled = true
	cfg.Hardware.Quality = "lossless"

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "hardware_transcode.quality") {
		t.Fatalf("expected hardware quality validation error, got %v", err)
	}
}
