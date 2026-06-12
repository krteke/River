package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/krteke/River/internal/config"
	filesystem "github.com/krteke/River/internal/fs"
	"github.com/krteke/River/internal/media"
	"github.com/krteke/River/internal/transcode"
)

func TestFileDisplayAndDownload(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "notes.txt"), []byte("hello"))
	mustWrite(t, filepath.Join(root, "archive.bin"), []byte("binary"))
	mustWrite(t, filepath.Join(root, "large.txt"), make([]byte, maxTextFileSize+1))
	handler, _ := testHandler(t, root)

	response := request(t, handler, "/api/file?root=media&path=/notes.txt")
	if response.Code != http.StatusOK || response.Body.String() != "hello" {
		t.Fatalf("unexpected text response: status=%d body=%q", response.Code, response.Body.String())
	}
	if disposition := response.Header().Get("Content-Disposition"); !strings.HasPrefix(disposition, "inline;") {
		t.Fatalf("unexpected inline disposition: %q", disposition)
	}

	response = request(t, handler, "/api/file?root=media&path=/archive.bin")
	assertAPIError(t, response, http.StatusUnsupportedMediaType, "unsupported_file_type")

	response = request(t, handler, "/api/file?root=media&path=/large.txt")
	assertAPIError(t, response, http.StatusRequestEntityTooLarge, "text_file_too_large")

	response = request(t, handler, "/api/download?root=media&path=/archive.bin")
	if response.Code != http.StatusOK || response.Body.String() != "binary" {
		t.Fatalf("unexpected download response: status=%d body=%q", response.Code, response.Body.String())
	}
	if disposition := response.Header().Get("Content-Disposition"); !strings.HasPrefix(disposition, "attachment;") {
		t.Fatalf("unexpected attachment disposition: %q", disposition)
	}
}

func TestVideoPlayDirect(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "movie.mp4"), []byte("video"))
	handler, _ := testHandler(t, root)

	response := request(t, handler, "/api/video/play?root=media&path=/movie.mp4&start_seconds=120")
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", response.Code, response.Body.String())
	}
	var body playResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Mode != "direct" || body.Mime != "video/mp4" || body.StartSeconds != 59.999 {
		t.Fatalf("unexpected play response: %+v", body)
	}
	if body.URL != "/api/file?path=%2Fmovie.mp4&root=media" {
		t.Fatalf("unexpected direct URL: %q", body.URL)
	}
}

func TestVideoPlayHLSAndServeStream(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "movie.mkv"), []byte("video"))
	handler, _ := testHandler(t, root)

	response := request(t, handler, "/api/video/play?root=media&path=/movie.mkv")
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected play status: %d body=%s", response.Code, response.Body.String())
	}
	var body playResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Mode != "hls" || body.SessionID == "" {
		t.Fatalf("unexpected HLS response: %+v", body)
	}

	for _, streamFile := range []struct {
		name        string
		contentType string
		contains    string
	}{
		{"master.m3u8", "application/vnd.apple.mpegurl", "index.m3u8"},
		{"index.m3u8", "application/vnd.apple.mpegurl", "seg_000000.ts"},
		{"seg_000000.ts", "video/mp2t", "segment"},
	} {
		response = request(t, handler, "/stream/"+body.SessionID+"/"+streamFile.name)
		if response.Code != http.StatusOK {
			t.Fatalf("%s returned %d: %s", streamFile.name, response.Code, response.Body.String())
		}
		if got := response.Header().Get("Content-Type"); got != streamFile.contentType {
			t.Fatalf("%s content type: got %q want %q", streamFile.name, got, streamFile.contentType)
		}
		if !strings.Contains(response.Body.String(), streamFile.contains) {
			t.Fatalf("%s has unexpected body: %q", streamFile.name, response.Body.String())
		}
	}

	stopRequest := httptest.NewRequest(http.MethodDelete, "/api/video/session/"+body.SessionID, nil)
	stopResponse := httptest.NewRecorder()
	handler.ServeHTTP(stopResponse, stopRequest)
	if stopResponse.Code != http.StatusNoContent {
		t.Fatalf("unexpected stop status: %d body=%s", stopResponse.Code, stopResponse.Body.String())
	}
	response = request(t, handler, "/stream/"+body.SessionID+"/master.m3u8")
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected stopped stream to return 404, got %d", response.Code)
	}
}

func TestVideoPlayValidationAndToolErrors(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "movie.mp4"), []byte("video"))
	handler, manager := testHandler(t, root)

	response := request(t, handler, "/api/video/play?root=media&path=/movie.mp4&start_seconds=invalid")
	assertAPIError(t, response, http.StatusBadRequest, "bad_request")
	response = request(t, handler, "/api/video/play?path=/movie.mp4")
	assertAPIError(t, response, http.StatusBadRequest, "bad_request")

	manager.StopAll()
	cfg := testConfig(t, root)
	cfg.FFmpeg.FFprobePath = filepath.Join(t.TempDir(), "missing-ffprobe")
	fileService, err := filesystem.NewService(cfg.Roots)
	if err != nil {
		t.Fatal(err)
	}
	missingProbeHandler := NewServer(fileService, media.NewService(cfg.FFmpeg.FFprobePath, cfg.Playback), transcode.NewManager(cfg)).Handler()
	response = request(t, missingProbeHandler, "/api/video/info?root=media&path=/movie.mp4")
	assertAPIError(t, response, http.StatusServiceUnavailable, "ffmpeg_not_available")

	mustWrite(t, filepath.Join(root, "movie.mkv"), []byte("video"))
	cfg = testConfig(t, root)
	cfg.FFmpeg.FFmpegPath = filepath.Join(t.TempDir(), "missing-ffmpeg")
	fileService, err = filesystem.NewService(cfg.Roots)
	if err != nil {
		t.Fatal(err)
	}
	missingFFmpegHandler := NewServer(fileService, media.NewService(cfg.FFmpeg.FFprobePath, cfg.Playback), transcode.NewManager(cfg)).Handler()
	response = request(t, missingFFmpegHandler, "/api/video/play?root=media&path=/movie.mkv")
	assertAPIError(t, response, http.StatusServiceUnavailable, "ffmpeg_not_available")
}

func testHandler(t *testing.T, root string) (http.Handler, *transcode.Manager) {
	t.Helper()
	cfg := testConfig(t, root)
	fileService, err := filesystem.NewService(cfg.Roots)
	if err != nil {
		t.Fatal(err)
	}
	manager := transcode.NewManager(cfg)
	t.Cleanup(manager.StopAll)
	return NewServer(fileService, media.NewService(cfg.FFmpeg.FFprobePath, cfg.Playback), manager).Handler(), manager
}

func testConfig(t *testing.T, root string) *config.Config {
	t.Helper()
	cfg := config.Default()
	cfg.Roots = []config.RootConfig{{ID: "media", Name: "Media", Path: root}}
	cfg.FFmpeg.FFprobePath = fakeFFprobe(t)
	cfg.FFmpeg.FFmpegPath = fakeFFmpeg(t)
	cfg.FFmpeg.MaxConcurrentJobs = 2
	cfg.Transcode.TempDir = t.TempDir()
	return &cfg
}

func fakeFFprobe(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffprobe")
	script := `#!/bin/sh
for last do :; done
case "$last" in
*.mp4) container='mov,mp4,m4a,3gp,3g2,mj2'; codec='h264' ;;
*) container='matroska,webm'; codec='hevc' ;;
esac
printf '{"format":{"format_name":"%s","duration":"60.000","bit_rate":"8000000","size":"1000","start_time":"0"},"streams":[{"index":0,"codec_name":"%s","codec_type":"video","width":1920,"height":1080},{"index":1,"codec_name":"aac","codec_type":"audio","channels":2}]}' "$container" "$codec"
`
	mustWriteExecutable(t, path, script)
	return path
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
	mustWriteExecutable(t, path, script)
	return path
}

func request(t *testing.T, handler http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertAPIError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("unexpected status: got %d want %d body=%s", response.Code, status, response.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != code {
		t.Fatalf("unexpected error code: got %q want %q", body["error"], code)
	}
}

func mustWrite(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustWriteExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}
