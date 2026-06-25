package api

import (
	"errors"
	"io/fs"
	"log/slog"
	"net/http"

	filesystem "github.com/krteke/River/internal/fs"
	"github.com/krteke/River/internal/media"
	"github.com/krteke/River/internal/thumbnail"
	"github.com/krteke/River/internal/transcode"
)

var (
	errBadRequest          = errors.New("bad request")
	errUnauthorized        = errors.New("unauthorized")
	errTextFileTooLarge    = errors.New("text file too large")
	errUnsupportedFileType = errors.New("unsupported file type")
)

func writeError(w http.ResponseWriter, err error) {
	var code string
	var status int
	var message string

	switch {
	case errors.Is(err, errUnauthorized):
		code = "unauthorized"
		status = http.StatusUnauthorized
		message = "missing or invalid password"
	case errors.Is(err, errBadRequest):
		code = "bad_request"
		status = http.StatusBadRequest
		message = err.Error()
	case errors.Is(err, filesystem.ErrPathForbidden), errors.Is(err, fs.ErrPermission):
		code = "path_forbidden"
		status = http.StatusForbidden
		message = "path is outside root or cannot be accessed"
	case errors.Is(err, filesystem.ErrRootNotFound), errors.Is(err, transcode.ErrSessionNotFound), errors.Is(err, fs.ErrNotExist):
		code = "not_found"
		status = http.StatusNotFound
		message = "path not found"
	case errors.Is(err, filesystem.ErrNotAFile), errors.Is(err, filesystem.ErrNotDirectory):
		code = "not_found"
		status = http.StatusNotFound
		message = "requested path has the wrong type"
	case errors.Is(err, filesystem.ErrNotVideo):
		code = "unsupported_file_type"
		status = http.StatusUnsupportedMediaType
		message = "file is not a supported video"
	case errors.Is(err, filesystem.ErrNoThumbnail):
		code = "unsupported_file_type"
		status = http.StatusUnsupportedMediaType
		message = "file does not support thumbnails"
	case errors.Is(err, errTextFileTooLarge):
		code = "text_file_too_large"
		status = http.StatusRequestEntityTooLarge
		message = "text file is too large to display"
	case errors.Is(err, errUnsupportedFileType):
		code = "unsupported_file_type"
		status = http.StatusUnsupportedMediaType
		message = "file type is not supported for inline display"
	case errors.Is(err, transcode.ErrQueueFull):
		code = "transcode_queue_full"
		status = http.StatusServiceUnavailable
		message = "too many transcode jobs"
	case errors.Is(err, transcode.ErrFFmpegNotAvailable), errors.Is(err, media.ErrFFprobeNotAvailable), errors.Is(err, thumbnail.ErrFFmpegNotAvailable):
		code = "ffmpeg_not_available"
		status = http.StatusServiceUnavailable
		message = "ffmpeg tools are not available"
	case errors.Is(err, transcode.ErrProfileNotFound):
		code = "bad_request"
		status = http.StatusBadRequest
		message = "profile not found"
	default:
		code = "internal_error"
		status = http.StatusInternalServerError
		message = "Internal Server Error"
	}

	slog.Error("request failed", "code", code, "message", message, "error", err)

	writeJson(w, status, map[string]string{"error": code, "message": message})
}
