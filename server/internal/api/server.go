package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

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
	logger           *slog.Logger
}

func NewServer(cfg config.Config, fileService *filesystem.Service, mediaService *media.Service, transcodeManager *transcode.Manager, logger *slog.Logger) *Server {
	return &Server{
		cfg:              cfg,
		fileService:      fileService,
		mediaService:     mediaService,
		transcodeManager: transcodeManager,
		logger:           logger,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.healthHandler)
	mux.HandleFunc("/api/roots", s.rootsHandler)

	return mux
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

func writeJson(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
