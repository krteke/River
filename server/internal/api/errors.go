package api

import (
	"errors"
	"io/fs"
	"net/http"

	filesystem "github.com/krteke/River/internal/fs"
)

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
	case errors.Is(err, filesystem.ErrNotAFile):
		code = "path_not_a_file"
		status = http.StatusNotFound
		message = "Path is not a file"
	case errors.Is(err, fs.ErrPermission):
		code = "permission_denied"
		status = http.StatusForbidden
		message = "Path permission denied"
	default:
		code = "internal_server_error"
		status = http.StatusInternalServerError
		message = "Internal Server Error"
	}

	writeJson(w, status, map[string]string{"error": code, "message": message})
}
