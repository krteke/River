package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/krteke/River/internal/config"
)

const (
	TypeDirectory = "directory"
	TypeImage     = "image"
	TypeText      = "text"
	TypeVideo     = "video"
	TypeOther     = "other"
)

var (
	ErrRootNotFound  = errors.New("root not found")
	ErrPathForbidden = errors.New("path is outside root")
	ErrNotDirectory  = errors.New("path is not a directory")
)

type Service struct {
	roots map[string]Root
}

type Root struct {
	ID       string
	Name     string
	Path     string
	RealPath string
}

type ResolvedPath struct {
	Root    Root
	RelPath string
	AbsPath string
	Info    os.FileInfo
}

type ListResponse struct {
	RootID string     `json:"root_id"`
	Path   string     `json:"path"`
	Parent string     `json:"parent"`
	Items  []ListItem `json:"items"`
}

type ListItem struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Type  string `json:"type"`
	Size  int64  `json:"size"`
	MTime int64  `json:"mtime"`
}

func NewService(rootConfigs []config.RootConfig) (*Service, error) {
	roots := make(map[string]Root, len(rootConfigs))

	for _, config := range rootConfigs {
		abs, err := filepath.Abs(config.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve path for root %s: %w", config.ID, err)
		}

		realPath, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve symlinks for root %s: %w", config.ID, err)
		}

		name := config.Name
		if name == "" {
			name = config.ID
		}

		roots[config.ID] = Root{
			ID:       config.ID,
			Name:     name,
			Path:     abs,
			RealPath: filepath.Clean(realPath),
		}
	}

	return &Service{roots: roots}, nil
}

func (s *Service) Roots() map[string]Root {
	return s.roots
}

// func (s *Service) ListRoots() []Root {
// 	roots := make([]Root, 0, len(s.roots))
// 	for _, root := range s.roots {
// 		roots = append(roots, root)
// 	}

// 	return roots
// }
