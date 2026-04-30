package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

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
	ErrRootNotFound = errors.New("root not found")
	ErrNotDirectory = errors.New("path is not a directory")
)

type Service struct {
	roots map[string]Root
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

func (s *Service) List(root string, path string) (*ListResponse, error) {
	resolved, err := s.resolve(root, path)
	if err != nil {
		return nil, err
	}
	if !resolved.Info.IsDir() {
		return nil, ErrNotDirectory
	}

	entries, err := os.ReadDir(resolved.AbsPath)
	if err != nil {
		return nil, err
	}
	items := make([]ListItem, 0, len(entries))

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		itemPath := JoinReal(resolved.AbsPath, entry.Name())
		items = append(items, ListItem{
			Name:  entry.Name(),
			Path:  itemPath,
			Type:  typeForDirEntry(entry),
			Size:  fileSize(info),
			MTime: info.ModTime().Unix(),
		})
	}

	listResponse := &ListResponse{
		RootID: root,
		Path:   resolved.RelPath,
		Parent: parentPath(resolved.RelPath),
		Items:  items,
	}

	return listResponse, nil
}

func (s *Service) resolve(root string, path string) (*ResolvedPath, error) {
	r, ok := s.roots[root]
	if !ok {
		return nil, ErrRootNotFound
	}

	relativePath := CleanPath(path)
	absPath := filepath.Join(r.RealPath, strings.TrimPrefix(relativePath, "/"))
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}

	resolvedPath := &ResolvedPath{
		Root:    r,
		RelPath: relativePath,
		AbsPath: absPath,
		Info:    info,
	}

	return resolvedPath, nil
}

func CleanPath(p string) string {
	if p == "" {
		return "/"
	}
	cleaned := path.Clean("/" + strings.TrimSpace(p))

	return cleaned
}

func JoinReal(base string, name string) string {
	return filepath.Join(base, name)
}

func typeForDirEntry(entry os.DirEntry) string {
	if entry.IsDir() {
		return TypeDirectory
	}
	return typeForName(entry.Name())
}

func typeForName(name string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	switch ext {
	case "jpg", "jpeg", "png", "webp", "gif", "bmp", "avif":
		return TypeImage
	case "txt", "md", "nfo", "json", "yaml", "yml", "xml", "srt", "ass", "log":
		return TypeText
	case "mp4", "mov", "mkv", "webm", "avi", "m2ts", "ts", "flv", "wmv":
		return TypeVideo
	default:
		return TypeOther
	}
}

func fileSize(info os.FileInfo) int64 {
	if info.IsDir() {
		return 0
	}
	return info.Size()
}

func parentPath(path string) string {
	p := filepath.Dir(path)
	cleanedPath := CleanPath(p)

	if cleanedPath == "/" {
		return ""
	}

	return cleanedPath
}

// func (s *Service) ListRoots() []Root {
// 	roots := make([]Root, 0, len(s.roots))
// 	for _, root := range s.roots {
// 		roots = append(roots, root)
// 	}

// 	return roots
// }
