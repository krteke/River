package stream

import (
	"errors"
	"io/fs"
	"net/http"
	"strings"

	"github.com/krteke/River/internal/transcode"
)

type Service struct {
	manager *transcode.Manager
}

func NewService(manager *transcode.Manager) *Service {
	return &Service{manager: manager}
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	file, err := s.manager.OpenStreamFile(r.PathValue("session"), name)
	if err != nil {
		switch {
		case errors.Is(err, transcode.ErrSessionNotFound), errors.Is(err, fs.ErrNotExist):
			http.NotFound(w, r)
		default:
			http.Error(w, "invalid stream request", http.StatusBadRequest)
		}
		return
	}
	defer file.Close()

	if strings.HasSuffix(name, ".m3u8") {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-cache")
	} else if strings.HasSuffix(name, ".m4s") || strings.HasSuffix(name, ".mp4") {
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Cache-Control", "no-store")
	} else {
		w.Header().Set("Content-Type", "video/mp2t")
		w.Header().Set("Cache-Control", "no-store")
	}
	info, err := file.Stat()
	if err != nil {
		http.Error(w, "failed to read stream file", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, name, info.ModTime(), file)
}
