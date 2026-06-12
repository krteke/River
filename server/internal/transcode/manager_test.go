package transcode

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/krteke/River/internal/config"
)

func TestManagerStartsAndServesHLSFiles(t *testing.T) {
	cfg := testConfig(t, fakeFFmpeg(t))
	manager := NewManager(cfg)
	t.Cleanup(manager.StopAll)

	session, err := manager.Start(context.Background(), StartOptions{
		RootID:     "media",
		RelPath:    "/movie.mkv",
		SourcePath: "/movie.mkv",
	})
	if err != nil {
		t.Fatal(err)
	}

	file, err := manager.OpenStreamFile(session.ID, "index.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(file)
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "seg_000000.ts") {
		t.Fatalf("unexpected playlist: %q", content)
	}

	if _, err := manager.OpenStreamFile(session.ID, "../movie.mkv"); !errors.Is(err, ErrInvalidStreamFile) {
		t.Fatalf("expected invalid stream file, got %v", err)
	}
}

func TestManagerEnforcesConcurrentJobLimit(t *testing.T) {
	cfg := testConfig(t, fakeFFmpeg(t))
	cfg.FFmpeg.MaxConcurrentJobs = 1
	manager := NewManager(cfg)
	t.Cleanup(manager.StopAll)

	if _, err := manager.Start(context.Background(), StartOptions{SourcePath: "/first.mkv"}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Start(context.Background(), StartOptions{SourcePath: "/second.mkv"}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected queue full, got %v", err)
	}
}

func TestOpenStreamFileRefreshesSessionAccess(t *testing.T) {
	cfg := testConfig(t, fakeFFmpeg(t))
	manager := NewManager(cfg)
	t.Cleanup(manager.StopAll)

	session, err := manager.Start(context.Background(), StartOptions{SourcePath: "/movie.mkv"})
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	manager.mu.Lock()
	session.LastAccessAt = old
	manager.mu.Unlock()

	file, err := manager.OpenStreamFile(session.ID, "master.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()

	manager.mu.Lock()
	lastAccess := session.LastAccessAt
	manager.mu.Unlock()
	if !lastAccess.After(old) {
		t.Fatalf("last access time was not refreshed: %v", lastAccess)
	}
}

func TestManagerReportsMissingFFmpeg(t *testing.T) {
	cfg := testConfig(t, filepath.Join(t.TempDir(), "missing-ffmpeg"))
	manager := NewManager(cfg)

	_, err := manager.Start(context.Background(), StartOptions{SourcePath: "/movie.mkv"})
	if !errors.Is(err, ErrFFmpegNotAvailable) {
		t.Fatalf("expected ffmpeg unavailable, got %v", err)
	}
}

func TestCleanupIdleRemovesSessionAndFiles(t *testing.T) {
	cfg := testConfig(t, fakeFFmpeg(t))
	manager := NewManager(cfg)
	t.Cleanup(manager.StopAll)

	session, err := manager.Start(context.Background(), StartOptions{SourcePath: "/movie.mkv"})
	if err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	session.LastAccessAt = time.Now().Add(-2 * cfg.IdleTimeout())
	manager.mu.Unlock()

	manager.cleanupIdle()
	if _, err := manager.OpenStreamFile(session.ID, "master.m3u8"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected removed session, got %v", err)
	}
	if _, err := os.Stat(session.TempDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected temp directory removal, got %v", err)
	}
}

func TestFFmpegArgsCreateIndependentKeyframeAlignedHLS(t *testing.T) {
	cfg := testConfig(t, "/usr/bin/ffmpeg")
	profile := cfg.Profiles[0]
	args := buildFFmpegArgs(cfg, profile, "/movie.mkv", "/tmp/session", 12.5)
	joined := strings.Join(args, " ")

	for _, expected := range []string{
		"-ss 12.500",
		"-force_key_frames expr:gte(t,n_forced*6)",
		"-hls_flags independent_segments",
		"-hls_segment_filename /tmp/session/seg_%06d.ts",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %q in ffmpeg args: %s", expected, joined)
		}
	}
}

func testConfig(t *testing.T, ffmpegPath string) *config.Config {
	t.Helper()
	cfg := config.Default()
	cfg.FFmpeg.FFmpegPath = ffmpegPath
	cfg.FFmpeg.MaxConcurrentJobs = 2
	cfg.Transcode.TempDir = t.TempDir()
	cfg.Transcode.IdleTimeoutSeconds = 60
	return &cfg
}

func fakeFFmpeg(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffmpeg")
	script := `#!/bin/sh
for last do :; done
dir=${last%/*}
printf 'segment' > "$dir/seg_000000.ts"
printf '#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXTINF:6,\nseg_000000.ts\n' > "$last"
exec sleep 30
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
