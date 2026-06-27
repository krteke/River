package transcode

import (
	"context"
	"errors"
	"fmt"
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

func TestManagerRejectsDirectProfileForTranscoding(t *testing.T) {
	cfg := testConfig(t, fakeFFmpeg(t))
	cfg.Profiles = append([]config.ProfileConfig{{Name: "original", Direct: true}}, cfg.Profiles...)
	manager := NewManager(cfg)

	_, err := manager.Start(context.Background(), StartOptions{SourcePath: "/movie.mp4", ProfileName: "original"})
	if !errors.Is(err, ErrDirectProfile) {
		t.Fatalf("expected direct profile error, got %v", err)
	}
}

func TestPlaybackOptionsExposeConfiguredProfiles(t *testing.T) {
	cfg := testConfig(t, "/usr/bin/ffmpeg")
	cfg.Profiles = []config.ProfileConfig{
		{Name: "original", Direct: true},
		{Name: "hevc_720p_3m", VideoCodec: "libx265", Width: 1280, VideoBitrate: "3000k"},
	}
	cfg.Playback.DefaultProfile = "hevc_720p_3m"
	manager := NewManager(cfg)

	options := manager.PlaybackOptions()
	if len(options) != 2 {
		t.Fatalf("expected two playback options, got %+v", options)
	}
	if !options[0].Direct || options[0].Label != "原画" {
		t.Fatalf("unexpected original option: %+v", options[0])
	}
	if options[1].Codec != "hevc" || options[1].Resolution != "720p" || options[1].Bitrate != "3000k" || !options[1].Default {
		t.Fatalf("unexpected transcode option: %+v", options[1])
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
	args := buildFFmpegArgs(cfg, profile, "/movie.mkv", "/tmp/session", 12.5, transcodeModeSoftware)
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

func TestNVENCFFmpegArgsUseBackendAndQuality(t *testing.T) {
	cfg := testConfig(t, "/usr/bin/ffmpeg")
	cfg.Hardware.Enabled = true
	cfg.Hardware.Backend = "nvenc"
	cfg.Hardware.Quality = "quality"
	profile := cfg.Profiles[0]

	args := buildFFmpegArgs(cfg, profile, "/movie.mkv", "/tmp/session", 0, transcodeModeHardware)
	joined := strings.Join(args, " ")

	for _, expected := range []string{
		"-i /movie.mkv",
		"-vf scale='min(1920,iw)':-2",
		"-c:v h264_nvenc",
		"-preset p7",
		"-pix_fmt yuv420p",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %q in ffmpeg args: %s", expected, joined)
		}
	}
}

func TestVAAPIFFmpegArgsUseBackendDefaults(t *testing.T) {
	cfg := testConfig(t, "/usr/bin/ffmpeg")
	cfg.Hardware.Enabled = true
	cfg.Hardware.Backend = "vaapi"
	cfg.Hardware.Device = "/dev/dri/renderD128"
	cfg.Hardware.Quality = "speed"
	profile := cfg.Profiles[0]

	args := buildFFmpegArgs(cfg, profile, "/movie.mkv", "/tmp/session", 0, transcodeModeHardware)
	joined := strings.Join(args, " ")

	for _, expected := range []string{
		"-vaapi_device /dev/dri/renderD128",
		"-hwaccel vaapi",
		"-hwaccel_output_format vaapi",
		"-i /movie.mkv",
		"-vf scale_vaapi=w=1920:h=-2:format=nv12",
		"-c:v h264_vaapi",
		"-quality 7",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %q in ffmpeg args: %s", expected, joined)
		}
	}
}

func TestHEVCSoftwareFFmpegArgsUseFMP4HLS(t *testing.T) {
	cfg := testConfig(t, "/usr/bin/ffmpeg")
	profile := hevcProfile()

	args := buildFFmpegArgs(cfg, profile, "/movie.mkv", "/tmp/session", 0, transcodeModeSoftware)
	joined := strings.Join(args, " ")

	for _, expected := range []string{
		"-c:v libx265",
		"-profile:v main",
		"-pix_fmt yuv420p",
		"-hls_segment_type fmp4",
		"-hls_fmp4_init_filename init.mp4",
		"-hls_segment_filename /tmp/session/seg_%06d.m4s",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %q in ffmpeg args: %s", expected, joined)
		}
	}
	if strings.Contains(joined, "-tag:v hvc1") {
		t.Fatalf("HEVC profile should not force hvc1 by default: %s", joined)
	}
}

func TestHEVCVAAPIFFmpegArgsSelectCodecFromProfile(t *testing.T) {
	cfg := testConfig(t, "/usr/bin/ffmpeg")
	cfg.Hardware.Enabled = true
	cfg.Hardware.Backend = "vaapi"
	cfg.Hardware.Device = "/dev/dri/renderD128"
	profile := hevcProfile()

	args := buildFFmpegArgs(cfg, profile, "/movie.mkv", "/tmp/session", 0, transcodeModeHardware)
	joined := strings.Join(args, " ")

	for _, expected := range []string{
		"-vaapi_device /dev/dri/renderD128",
		"-hwaccel vaapi",
		"-vf scale_vaapi=w=1920:h=-2:format=nv12",
		"-c:v hevc_vaapi",
		"-profile:v main",
		"-rc_mode VBR",
		"-bf 0",
		"-idr_interval 1",
		"-hls_segment_type fmp4",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %q in ffmpeg args: %s", expected, joined)
		}
	}
	if strings.Contains(joined, "-tag:v hvc1") {
		t.Fatalf("HEVC VAAPI profile should not force hvc1 by default: %s", joined)
	}
}

func TestExplicitVideoTagIsPreserved(t *testing.T) {
	cfg := testConfig(t, "/usr/bin/ffmpeg")
	profile := hevcProfile()
	profile.VideoTag = "hvc1"

	args := buildFFmpegArgs(cfg, profile, "/movie.mkv", "/tmp/session", 0, transcodeModeSoftware)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-tag:v hvc1") {
		t.Fatalf("expected explicit video tag to be preserved: %s", joined)
	}
}

func TestValidStreamFileAllowsFMP4HLSFiles(t *testing.T) {
	for _, name := range []string{"init.mp4", "seg_000000.m4s", "seg_123456.m4s"} {
		if !validStreamFile(name) {
			t.Fatalf("expected %s to be valid", name)
		}
	}
	for _, name := range []string{"seg_1.m4s", "seg_abcdef.m4s", "../init.mp4"} {
		if validStreamFile(name) {
			t.Fatalf("expected %s to be invalid", name)
		}
	}
}

func TestManagerFallsBackToSoftwareWhenHardwareFails(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "ffmpeg.log")
	cfg := testConfig(t, fakeFFmpegWithHardwareFailure(t, logPath))
	cfg.Hardware.Enabled = true
	cfg.Hardware.Backend = "nvenc"
	cfg.Hardware.FallbackToSoftware = true
	manager := NewManager(cfg)
	t.Cleanup(manager.StopAll)

	session, err := manager.Start(context.Background(), StartOptions{SourcePath: "/movie.mkv"})
	if err != nil {
		t.Fatal(err)
	}
	if session.mode != transcodeModeSoftware {
		t.Fatalf("expected software fallback mode, got %s", session.mode)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	joined := string(content)
	if !strings.Contains(joined, "h264_nvenc") || !strings.Contains(joined, "libx264") {
		t.Fatalf("expected hardware and software ffmpeg attempts, got %s", joined)
	}
}

func TestManagerAutoHardwareTriesNextBackendBeforeSoftware(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "ffmpeg.log")
	cfg := testConfig(t, fakeFFmpegWithVAAPIFailure(t, logPath))
	cfg.Hardware.Enabled = true
	cfg.Hardware.Backend = "auto"
	cfg.Hardware.FallbackToSoftware = true
	manager := NewManager(cfg)
	t.Cleanup(manager.StopAll)

	session, err := manager.Start(context.Background(), StartOptions{SourcePath: "/movie.mkv"})
	if err != nil {
		t.Fatal(err)
	}
	if session.mode != transcodeModeHardware || session.backend != hardwareBackendNVENC {
		t.Fatalf("expected nvenc hardware fallback, got mode=%s backend=%s", session.mode, session.backend)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	joined := string(content)
	if !strings.Contains(joined, "h264_vaapi") || !strings.Contains(joined, "h264_nvenc") || strings.Contains(joined, "libx264") {
		t.Fatalf("expected vaapi then nvenc without software fallback, got %s", joined)
	}
}

func TestManagerFallsBackWhenHardwareOutputIsInvalid(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "ffmpeg.log")
	cfg := testConfig(t, fakeFFmpegWithInvalidHardwareOutput(t, logPath))
	cfg.Hardware.Enabled = true
	cfg.Hardware.Backend = "vaapi"
	cfg.Hardware.FallbackToSoftware = true
	manager := NewManager(cfg)
	t.Cleanup(manager.StopAll)

	session, err := manager.Start(context.Background(), StartOptions{SourcePath: "/movie.mkv"})
	if err != nil {
		t.Fatal(err)
	}
	if session.mode != transcodeModeSoftware {
		t.Fatalf("expected software fallback after invalid hardware output, got %s", session.mode)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	joined := string(content)
	if !strings.Contains(joined, "h264_vaapi") || !strings.Contains(joined, "libx264") || !strings.Contains(joined, "-f null -") {
		t.Fatalf("expected hardware attempt, validation, and software fallback, got %s", joined)
	}
}

func hevcProfile() config.ProfileConfig {
	return config.ProfileConfig{
		Name:            "hevc_1080p_5m",
		Container:       "hls_fmp4",
		VideoCodec:      "libx265",
		VideoProfile:    "main",
		PixelFormat:     "yuv420p",
		Width:           1920,
		VideoBitrate:    "5000k",
		AudioCodec:      "aac",
		AudioBitrate:    "160k",
		AudioChannels:   2,
		Preset:          "veryfast",
		SegmentDuration: 6,
	}
}

func testConfig(t *testing.T, ffmpegPath string) *config.Config {
	t.Helper()
	cfg := config.Default()
	profiles := make([]config.ProfileConfig, 0, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		if !profile.Direct {
			profiles = append(profiles, profile)
		}
	}
	cfg.Profiles = profiles
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
dir=${last%%/*}
printf 'segment' > "$dir/seg_000000.ts"
printf '#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXTINF:6,\nseg_000000.ts\n' > "$last"
exec sleep 30
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func fakeFFmpegWithHardwareFailure(t *testing.T, logPath string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffmpeg")
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> %q
case "$*" in
*"-f null -"*)
  exit 0
  ;;
*h264_nvenc*)
  echo 'hardware encoder failed' >&2
  exit 1
  ;;
esac
for last do :; done
dir=${last%%/*}
printf 'segment' > "$dir/seg_000000.ts"
printf '#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXTINF:6,\nseg_000000.ts\n' > "$last"
exec sleep 30
`, logPath)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func fakeFFmpegWithVAAPIFailure(t *testing.T, logPath string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffmpeg")
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> %q
case "$*" in
*"-f null -"*)
  exit 0
  ;;
*h264_vaapi*)
  echo 'vaapi encoder failed' >&2
  exit 1
  ;;
esac
for last do :; done
dir=${last%%/*}
printf 'segment' > "$dir/seg_000000.ts"
printf '#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXTINF:6,\nseg_000000.ts\n' > "$last"
exec sleep 30
`, logPath)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func fakeFFmpegWithInvalidHardwareOutput(t *testing.T, logPath string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffmpeg")
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> %q
case "$*" in
*"-f null -"*)
  echo '[hevc @ test] Skipping invalid undecodable NALU: 1' >&2
  exit 0
  ;;
esac
for last do :; done
dir=${last%%/*}
printf 'segment' > "$dir/seg_000000.ts"
printf '#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXTINF:6,\nseg_000000.ts\n' > "$last"
exec sleep 30
`, logPath)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
