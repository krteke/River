package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	filesystem "github.com/krteke/River/internal/fs"
	"github.com/krteke/River/internal/media"
	streaming "github.com/krteke/River/internal/stream"
	"github.com/krteke/River/internal/thumbnail"
	"github.com/krteke/River/internal/transcode"
)

const maxTextFileSize = 2 << 20
const authPasswordHeader = "X-River-Password"

type Server struct {
	fileService      *filesystem.Service
	mediaService     *media.Service
	thumbnailService *thumbnail.Service
	transcodeManager *transcode.Manager
	streamService    *streaming.Service
	password         string
}

type playResponse struct {
	Mode            string                `json:"mode"`
	URL             string                `json:"url"`
	Mime            string                `json:"mime,omitempty"`
	SessionID       string                `json:"session_id,omitempty"`
	Profile         string                `json:"profile,omitempty"`
	StartSeconds    float64               `json:"start_seconds,omitempty"`
	DurationSeconds float64               `json:"duration_seconds,omitempty"`
	Subtitles       []media.SubtitleTrack `json:"subtitles,omitempty"`
}

func NewServer(fileService *filesystem.Service, mediaService *media.Service, thumbnailService *thumbnail.Service, transcodeManager *transcode.Manager, password string) *Server {
	return &Server{
		fileService:      fileService,
		mediaService:     mediaService,
		thumbnailService: thumbnailService,
		transcodeManager: transcodeManager,
		streamService:    streaming.NewService(transcodeManager),
		password:         password,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.healthHandler)
	mux.HandleFunc("GET /api/roots", s.rootsHandler)
	mux.HandleFunc("GET /api/list", s.listHandler)
	mux.HandleFunc("GET /api/file", s.fileHandler)
	mux.HandleFunc("GET /api/thumbnail", s.thumbnailHandler)
	mux.HandleFunc("GET /api/download", s.downloadHandler)
	mux.HandleFunc("GET /api/video/info", s.videoInfoHandler)
	mux.HandleFunc("GET /api/video/subtitle", s.videoSubtitleHandler)
	mux.HandleFunc("GET /api/video/playback-options", s.videoPlaybackOptionsHandler)
	mux.HandleFunc("GET /api/video/play", s.videoPlayHandler)
	mux.HandleFunc("DELETE /api/video/session/{session}", s.videoStopHandler)
	mux.Handle("GET /stream/{session}/{file}", s.streamService)

	return s.withLog(s.withAuth(mux))
}

// example:
// curl "localhost:8080/api/health"
//
// {"ok":true}
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJson(w, http.StatusOK, map[string]bool{"ok": true})
}

// example:
// curl "localhost:8080/api/roots"
//
// [{"id":"data","name":"Data"}]
func (s *Server) rootsHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("received roots request")

	type RootResponse struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	roots := s.fileService.Roots()

	rootResponses := make([]RootResponse, len(roots))
	for i, root := range roots {
		rootResponses[i] = RootResponse{
			ID:   root.ID,
			Name: root.Name,
		}
	}

	writeJson(w, http.StatusOK, rootResponses)
}

// example:
// curl "localhost:8080/api/list?root=data&path=path/to/files"
func (s *Server) listHandler(w http.ResponseWriter, r *http.Request) {
	root, path, err := parseQuery(r, false)
	if err != nil {
		writeError(w, err)
		return
	}

	list, err := s.fileService.List(root, path)
	if err != nil {
		slog.Error("failed to list", "root", root, "path", path, "error", err)
		writeError(w, err)

		return
	}

	slog.Info("list directory", "root", root, "path", path, "items", len(list.Items))
	for i := range list.Items {
		switch list.Items[i].Type {
		case filesystem.TypeImage, filesystem.TypeVideo:
			list.Items[i].ThumbnailURL = thumbnailURL(root, list.Items[i].Path)
		}
	}
	writeJson(w, http.StatusOK, list)
}

func (s *Server) fileHandler(w http.ResponseWriter, r *http.Request) {
	root, path, err := parseQuery(r, true)
	if err != nil {
		writeError(w, err)
		return
	}

	file, info, err := s.fileService.File(root, path)
	if err != nil {
		slog.Error("failed to get file", "root", root, "path", path, "error", err)
		writeError(w, err)
		return
	}
	defer file.Close()

	if info.Type == filesystem.TypeOther {
		writeError(w, errUnsupportedFileType)
		return
	}
	if info.Type == filesystem.TypeText && info.Size > maxTextFileSize {
		writeError(w, errTextFileTooLarge)
		return
	}
	w.Header().Set("Content-Type", info.Mime)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": info.Name}))
	slog.Info("read file", "root", root, "path", path, "type", info.Type)
	http.ServeContent(w, r, info.Name, info.ModTime, file)
}

func (s *Server) thumbnailHandler(w http.ResponseWriter, r *http.Request) {
	root, path, err := parseQuery(r, true)
	if err != nil {
		writeError(w, err)
		return
	}
	resolved, err := s.fileService.ResolveThumbnailSource(root, path)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := s.thumbnailService.Get(r.Context(), resolved)
	if err != nil {
		slog.Error("failed to generate thumbnail", "root", root, "path", path, "error", err)
		writeError(w, err)
		return
	}
	file, err := os.Open(result.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	slog.Info("read thumbnail", "root", root, "path", path)
	http.ServeContent(w, r, "thumbnail.jpg", result.ModTime, file)
}

func (s *Server) downloadHandler(w http.ResponseWriter, r *http.Request) {
	root, path, err := parseQuery(r, true)
	if err != nil {
		writeError(w, err)
		return
	}
	file, info, err := s.fileService.File(root, path)
	if err != nil {
		writeError(w, err)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": info.Name}))
	slog.Info("download file", "root", root, "path", path)
	http.ServeContent(w, r, info.Name, info.ModTime, file)
}

func (s *Server) videoInfoHandler(w http.ResponseWriter, r *http.Request) {
	root, path, err := parseQuery(r, true)
	if err != nil {
		writeError(w, err)
		return
	}
	_, info, err := s.mediaInfo(r.Context(), root, path)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJson(w, http.StatusOK, info)
}

func (s *Server) videoSubtitleHandler(w http.ResponseWriter, r *http.Request) {
	root, path, err := parseQuery(r, true)
	if err != nil {
		writeError(w, err)
		return
	}
	trackIndex, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("track")))
	if err != nil || trackIndex < 0 {
		writeError(w, errBadRequest)
		return
	}
	resolved, info, err := s.mediaInfo(r.Context(), root, path)
	if err != nil {
		writeError(w, err)
		return
	}
	track, ok := subtitleTrack(info, trackIndex)
	if !ok {
		writeError(w, fs.ErrNotExist)
		return
	}

	content, err := s.mediaService.ExtractSubtitle(r.Context(), resolved.AbsPath, track)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (s *Server) videoPlaybackOptionsHandler(w http.ResponseWriter, r *http.Request) {
	writeJson(w, http.StatusOK, s.transcodeManager.PlaybackOptions())
}

func (s *Server) videoPlayHandler(w http.ResponseWriter, r *http.Request) {
	root, path, err := parseQuery(r, true)
	if err != nil {
		writeError(w, err)
		return
	}
	resolved, info, err := s.mediaInfo(r.Context(), root, path)
	if err != nil {
		writeError(w, err)
		return
	}

	startSeconds, err := parseStartSeconds(r)
	if err != nil {
		writeError(w, err)
		return
	}
	startSeconds = clampStartSeconds(startSeconds, info.Container.Duration)

	profileName := strings.TrimSpace(r.URL.Query().Get("profile"))
	var selectedProfileName string
	if profileName != "" {
		profile, ok := s.transcodeManager.Profile(profileName)
		if !ok {
			writeError(w, transcode.ErrProfileNotFound)
			return
		}
		selectedProfileName = profile.Name
		if profile.Direct {
			s.writeDirectPlay(w, root, path, resolved, info, startSeconds, profile.Name)
			return
		}
	}

	playbackMode := s.mediaService.PlaybackMode(info)

	if profileName == "" && playbackMode == media.PlaybackModeDirect {
		s.writeDirectPlay(w, root, path, resolved, info, startSeconds, "")
		return
	}
	if playbackMode == media.PlaybackModeUnsupported {
		writeError(w, errUnsupportedFileType)
		return
	}

	session, err := s.transcodeManager.Start(r.Context(), transcode.StartOptions{
		RootID:           root,
		RelPath:          resolved.RelPath,
		SourcePath:       resolved.AbsPath,
		ProfileName:      selectedProfileName,
		StartSeconds:     startSeconds,
		ReplaceSessionID: strings.TrimSpace(r.URL.Query().Get("replace_session_id")),
	})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJson(w, http.StatusOK, playResponse{
		Mode:            "hls",
		SessionID:       session.ID,
		URL:             "/stream/" + session.ID + "/master.m3u8",
		Profile:         session.ProfileName,
		StartSeconds:    session.StartSeconds,
		DurationSeconds: info.Container.Duration,
		Subtitles:       info.Tracks.Subtitle,
	})
}

func (s *Server) writeDirectPlay(w http.ResponseWriter, root, path string, resolved *filesystem.ResolvedPath, info *media.MediaInfo, startSeconds float64, profileName string) {
	slog.Info("play direct", "root", root, "path", path, "profile", profileName)
	writeJson(w, http.StatusOK, playResponse{
		Mode:            "direct",
		URL:             fileURL(root, resolved.RelPath),
		Mime:            filesystem.ContentType(resolved.AbsPath),
		Profile:         profileName,
		StartSeconds:    startSeconds,
		DurationSeconds: info.Container.Duration,
		Subtitles:       info.Tracks.Subtitle,
	})
}

func subtitleTrack(info *media.MediaInfo, index int) (media.SubtitleTrack, bool) {
	for _, track := range info.Tracks.Subtitle {
		if track.Index == index {
			return track, true
		}
	}
	return media.SubtitleTrack{}, false
}

func (s *Server) videoStopHandler(w http.ResponseWriter, r *http.Request) {
	if !s.transcodeManager.Stop(r.PathValue("session")) {
		writeError(w, transcode.ErrSessionNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) withLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("request", "method", r.Method, "url", r.URL.String())

		next.ServeHTTP(w, r)
	})
}

func (s *Server) withAuth(next http.Handler) http.Handler {
	if s.password == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if constantTimeEqual(r.Header.Get(authPasswordHeader), s.password) {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, errUnauthorized)
	})
}

func constantTimeEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func (s *Server) mediaInfo(ctx context.Context, root, path string) (*filesystem.ResolvedPath, *media.MediaInfo, error) {
	resolved, err := s.fileService.ResolveVideo(root, path)
	if err != nil {
		slog.Error("failed to resolve video", "root", root, "path", path, "error", err)
		return nil, nil, err
	}

	slog.Info("probe video", "root", root, "path", path)
	info, err := s.mediaService.Probe(ctx, resolved.AbsPath)
	if err != nil {
		slog.Error("failed to probe video", "root", root, "path", path, "error", err)
		return nil, nil, err
	}

	return resolved, info, nil
}

func fileURL(root string, relPath string) string {
	values := url.Values{}
	values.Set("root", root)
	values.Set("path", relPath)
	return "/api/file?" + values.Encode()
}

func thumbnailURL(root string, relPath string) string {
	values := url.Values{}
	values.Set("root", root)
	values.Set("path", relPath)
	return "/api/thumbnail?" + values.Encode()
}

func clampStartSeconds(start, duration float64) float64 {
	if duration <= 0 {
		return start
	}
	if start < duration {
		return start
	}
	if duration <= 0.001 {
		return 0
	}

	return duration - 0.001
}

func parseQuery(r *http.Request, requirePath bool) (string, string, error) {
	root := strings.TrimSpace(r.URL.Query().Get("root"))
	path := r.URL.Query().Get("path")
	if root == "" {
		return "", "", fmt.Errorf("%w: root is required", errBadRequest)
	}
	if requirePath && path == "" {
		return "", "", fmt.Errorf("%w: path is required", errBadRequest)
	}
	return root, path, nil
}

func parseStartSeconds(r *http.Request) (float64, error) {
	start := r.URL.Query().Get("start_seconds")
	if start == "" {
		return 0, nil
	}
	seconds, err := strconv.ParseFloat(start, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid start_seconds", errBadRequest)
	}
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return 0, fmt.Errorf("%w: start_seconds must be a finite number", errBadRequest)
	}
	if seconds < 0 {
		return 0, fmt.Errorf("%w: start_seconds must be non-negative", errBadRequest)
	}

	return seconds, nil
}

func writeJson(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
