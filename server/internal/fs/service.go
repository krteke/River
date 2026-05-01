package filesystem

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
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
	ErrNotAFile     = errors.New("path is not a file")
	ErrNotVideo     = errors.New("file is not a video")
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

		var itemType string
		if entry.IsDir() {
			itemType = TypeDirectory
		} else {
			itemType = typeForFile(resolved.AbsPath)
		}

		items = append(items, ListItem{
			Name:  entry.Name(),
			Path:  itemPath,
			Type:  itemType,
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

func (s *Service) File(root string, path string) (*os.File, *FileInfo, error) {
	resolved, err := s.resolve(root, path)
	if err != nil {
		return nil, nil, err
	}

	if resolved.Info.IsDir() {
		return nil, nil, ErrNotAFile
	}

	file, err := os.Open(resolved.AbsPath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	info := &FileInfo{
		Mime:    ContentType(resolved.AbsPath),
		Name:    resolved.Info.Name(),
		ModTime: resolved.Info.ModTime(),
	}

	return file, info, nil
}

func (s *Service) ResolveVideo(root string, path string) (*ResolvedPath, error) {
	resolved, err := s.resolve(root, path)
	if err != nil {
		return nil, err
	}
	if resolved.Info.IsDir() {
		return nil, ErrNotAFile
	}
	if typeForFile(resolved.AbsPath) != TypeVideo {
		return nil, nil
	}

	return resolved, nil
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

func ContentType(path string) string {
	mtype, err := mimetype.DetectFile(path)
	if err == nil && mtype != nil {
		return mtype.String()
	} else {
		slog.Warn("failed to detect mime type", "path", path, "err", err)
	}

	return "application/octet-stream"
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

func typeForFile(path string) string {
	mtype := ContentType(path)

	switch strings.Split(mtype, "/")[0] {
	case "image":
		return TypeImage
	case "text":
		return TypeText
	case "video":
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
