package api

import (
	"encoding/json"
	"errors"
	"io/fs"
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
	mux.HandleFunc("/api/list", s.listHandler)

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
	s.logger.Info("received roots request")

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

func (s *Server) listHandler(w http.ResponseWriter, r *http.Request) {
	root := r.URL.Query().Get("root")
	path := r.URL.Query().Get("path")

	s.logger.Info("received list request", "root", root, "path", path)

	list, err := s.fileService.List(root, path)
	if err != nil {
		s.logger.Error("failed to list", "root", root, "path", path, "error", err)
		s.writeError(w, err)

		return
	}

	writeJson(w, http.StatusOK, list)
}

func (s *Server) writeError(w http.ResponseWriter, err error) {
	var code string
	var status int
	var message string

	switch {
	case errors.Is(err, filesystem.ErrRootNotFound):
		code = "root_not_found"
		status = http.StatusNotFound
		message = "Root not found"
	case errors.Is(err, fs.ErrNotExist):
		code = "path_not_exist"
		status = http.StatusNotFound
		message = "Path not exist"
	default:
		code = "internal_server_error"
		status = http.StatusInternalServerError
		message = "Internal Server Error"
	}

	writeJson(w, status, map[string]string{"error": code, "message": message})
}

func writeJson(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
