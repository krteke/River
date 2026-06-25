package thumbnail

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/krteke/River/internal/config"
)

func TestCleanupRemovesExpiredAndOldestOversizedFiles(t *testing.T) {
	cacheDir := t.TempDir()
	expired := filepath.Join(cacheDir, "expired.jpg")
	oldest := filepath.Join(cacheDir, "oldest.jpg")
	newest := filepath.Join(cacheDir, "newest.jpg")
	mustWrite(t, expired, 4)
	mustWrite(t, oldest, 700*1024)
	mustWrite(t, newest, 700*1024)

	now := time.Now()
	mustChtimes(t, expired, now.Add(-2*time.Hour))
	mustChtimes(t, oldest, now.Add(-20*time.Minute))
	mustChtimes(t, newest, now.Add(-10*time.Minute))

	service := NewService("ffmpeg", config.ThumbnailConfig{
		CacheDir:               cacheDir,
		Width:                  320,
		Height:                 180,
		VideoSeekSeconds:       10,
		MaxAgeSeconds:          60 * 60,
		MaxSizeMB:              1,
		CleanupIntervalSeconds: 60,
	})

	if err := service.Cleanup(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(expired); !os.IsNotExist(err) {
		t.Fatalf("expected expired file to be removed, got %v", err)
	}
	if _, err := os.Stat(oldest); !os.IsNotExist(err) {
		t.Fatalf("expected oldest oversized file to be removed, got %v", err)
	}
	if _, err := os.Stat(newest); err != nil {
		t.Fatalf("expected newest file to remain, got %v", err)
	}
}

func mustWrite(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustChtimes(t *testing.T, path string, when time.Time) {
	t.Helper()
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}
