package api

import (
	"errors"
	"io/fs"
	"log/slog"
	"net/http"

	filesystem "github.com/krteke/River/internal/fs"
	"github.com/krteke/River/internal/transcode"
)

func writeError(w http.ResponseWriter, err error) {
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
	case errors.Is(err, filesystem.ErrNotAFile):
		code = "path_not_a_file"
		status = http.StatusNotFound
		message = "Path is not a file"
	case errors.Is(err, filesystem.ErrNotVideo):
		code = "path_not_video"
		status = http.StatusNotFound
		message = "Path is not a video"
	case errors.Is(err, fs.ErrPermission):
		code = "permission_denied"
		status = http.StatusForbidden
		message = "Path permission denied"
	case errors.Is(err, transcode.ErrQueueFull):
		code = "transcode_queue_full"
		status = http.StatusUnavailableForLegalReasons
		message = "too many transcode jobs"
	case errors.Is(err, transcode.ErrProfileNotFound):
		code = "bad_request"
		status = http.StatusBadRequest
		message = "profile not found"
	default:
		code = "internal_server_error"
		status = http.StatusInternalServerError
		message = "Internal Server Error"
	}

	slog.Error("error", "code", code, "message", message)

	writeJson(w, status, map[string]string{"error": code, "message": message})
}
