package api

import (
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

	return nil
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
