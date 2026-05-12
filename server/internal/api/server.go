package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/krteke/River/internal/config"
	filesystem "github.com/krteke/River/internal/fs"
	"github.com/krteke/River/internal/media"
	"github.com/krteke/River/internal/transcode"
)

type Server struct {
	cfg              config.Config
	fileService      *filesystem.Service
	mediaService     *media.Service
	transcodeManager *transcode.Manager
}

type playResponse struct {
	Mode         string  `json:"mode"`
	URL          string  `json:"url"`
	Mime         string  `json:"mime,omitempty"`
	SessionID    string  `json:"session_id,omitempty"`
	Profile      string  `json:"profile,omitempty"`
	StartSeconds float64 `json:"start_seconds,omitempty"`
}

func NewServer(cfg config.Config, fileService *filesystem.Service, mediaService *media.Service, transcodeManager *transcode.Manager) *Server {
	return &Server{
		cfg:              cfg,
		fileService:      fileService,
		mediaService:     mediaService,
		transcodeManager: transcodeManager,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.healthHandler)
	mux.HandleFunc("GET /api/roots", s.rootsHandler)
	mux.HandleFunc("GET /api/list", s.listHandler)
	mux.HandleFunc("GET /api/file", s.fileHandler)
	mux.HandleFunc("GET /api/video/info", s.videoInfoHandler)
	mux.HandleFunc("GET /api/video/play", s.videoPlayHandler)

	return s.withLog(mux)
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
	i := 0
	for _, root := range roots {
		rootResponses[i] = RootResponse{
			ID:   root.ID,
			Name: root.Name,
		}
		i++
	}

	writeJson(w, http.StatusOK, rootResponses)
}

// example:
// curl "localhost:8080/api/list?root=data&path=path/to/files"
func (s *Server) listHandler(w http.ResponseWriter, r *http.Request) {
	root, path := parseQuery(r)

	list, err := s.fileService.List(root, path)
	if err != nil {
		slog.Error("failed to list", "root", root, "path", path, "error", err)
		writeError(w, err)

		return
	}

	writeJson(w, http.StatusOK, list)
}

func (s *Server) fileHandler(w http.ResponseWriter, r *http.Request) {
	root, path := parseQuery(r)

	file, info, err := s.fileService.File(root, path)
	if err != nil {
		slog.Error("failed to get file", "root", root, "path", path, "error", err)
		writeError(w, err)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", info.Mime)
	http.ServeContent(w, r, info.Name, info.ModTime, file)
}

func (s *Server) videoInfoHandler(w http.ResponseWriter, r *http.Request) {
	info, err := s.mediaInfo(r)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJson(w, http.StatusOK, info)
}

func (s *Server) videoPlayHandler(w http.ResponseWriter, r *http.Request) {
	root, path := parseQuery(r)
	resolved, err := s.fileService.ResolvedPath(root, path)

	info, err := s.mediaInfo(r)
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

	playback := s.mediaService.PlaybackInfo(info)

	if playback.Mode == media.PlaybackModeDirect {
		writeJson(w, http.StatusOK, playResponse{
			Mode:         "direct",
			URL:          fileURL(root, resolved.RelPath),
			Mime:         filesystem.ContentType(resolved.AbsPath),
			StartSeconds: startSeconds,
		})
		return
	}

	session, err := s.transcodeManager.Start(r.Context(), transcode.StartOptions{
		RootID:           root,
		RelPath:          resolved.RelPath,
		SourcePath:       resolved.AbsPath,
		ProfileName:      r.URL.Query().Get("profile"),
		StartSeconds:     startSeconds,
		ReplaceSessionID: strings.TrimSpace(r.URL.Query().Get("replace_session_id")),
	})
	if err != nil {
		writeError(w, err)
	}

	writeJson(w, http.StatusOK, playResponse{
		Mode:         "hls",
		SessionID:    session.ID,
		URL:          "/stream/" + session.ID + "/master.m3u8",
		Profile:      session.ProfileName,
		StartSeconds: session.StartSeconds,
	})
}

func (s *Server) withLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("request", "method", r.Method, "url", r.URL.String())

		next.ServeHTTP(w, r)
	})
}

func (s *Server) mediaInfo(r *http.Request) (*media.MediaInfo, error) {
	root, path := parseQuery(r)
	resolved, err := s.fileService.ResolveVideo(root, path)
	if err != nil {
		slog.Error("failed to resolve video", "root", root, "path", path, "error", err)
		return nil, err
	}

	info, err := s.mediaService.Probe(r.Context(), resolved.AbsPath)
	if err != nil {
		slog.Error("failed to probe video", "root", root, "path", path, "error", err)
		return nil, err
	}

	return info, nil
}

func fileURL(root string, relPath string) string {
	values := url.Values{}
	values.Set("root", root)
	values.Set("path", relPath)
	return "/api/file?" + values.Encode()
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

func parseQuery(r *http.Request) (string, string) {
	root := r.URL.Query().Get("root")
	path := r.URL.Query().Get("path")
	slog.Info("received list request", "root", root, "path", path)

	return root, path
}

func parseStartSeconds(r *http.Request) (float64, error) {
	start := r.URL.Query().Get("start_seconds")
	if start == "" {
		return 0, nil
	}
	seconds, err := strconv.ParseFloat(start, 64)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return 0, errors.New("start_seconds must be a finite number")
	}
	if seconds < 0 {
		return 0, errors.New("start_seconds must be non-negative")
	}

	return seconds, nil
}

func writeJson(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
