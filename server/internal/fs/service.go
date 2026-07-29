package filesystem

import (
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
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
	ErrRootNotFound  = errors.New("root not found")
	ErrNotDirectory  = errors.New("path is not a directory")
	ErrNotAFile      = errors.New("path is not a file")
	ErrNoThumbnail   = errors.New("file does not support thumbnails")
	ErrPathForbidden = errors.New("path is outside root")
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
			RealPath: filepath.Clean(realPath),
		}
		info, err := os.Stat(realPath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat root %s: %w", config.ID, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("root %s is not a directory", config.ID)
		}
	}

	return &Service{roots: roots}, nil
}

func (s *Service) Roots() []Root {
	roots := make([]Root, 0, len(s.roots))
	for _, root := range s.roots {
		roots = append(roots, root)
	}
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].ID < roots[j].ID
	})
	return roots
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
		itemPath := pathpkg.Join(resolved.RelPath, entry.Name())
		itemResolved, err := s.resolve(root, itemPath)
		if err != nil {
			continue
		}
		info := itemResolved.Info

		var itemType string
		if info.IsDir() {
			itemType = TypeDirectory
		} else {
			itemType = TypeForFile(itemResolved.AbsPath)
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

	info := &FileInfo{
		Mime:    ContentType(resolved.AbsPath),
		Name:    resolved.Info.Name(),
		Type:    TypeForFile(resolved.AbsPath),
		Size:    resolved.Info.Size(),
		ModTime: resolved.Info.ModTime(),
	}

	return file, info, nil
}

func (s *Service) ResolveFile(root string, path string) (*ResolvedPath, error) {
	resolved, err := s.resolve(root, path)
	if err != nil {
		return nil, err
	}
	if resolved.Info.IsDir() {
		return nil, ErrNotAFile
	}

	return resolved, nil
}

func (s *Service) ResolveThumbnailSource(root string, path string) (*ResolvedPath, error) {
	resolved, err := s.resolve(root, path)
	if err != nil {
		return nil, err
	}
	if resolved.Info.IsDir() {
		return nil, ErrNotAFile
	}
	fileType := TypeForFile(resolved.AbsPath)
	if fileType != TypeImage && fileType != TypeVideo {
		return nil, ErrNoThumbnail
	}

	return resolved, nil
}

func (s *Service) resolve(root string, path string) (*ResolvedPath, error) {
	r, ok := s.roots[root]
	if !ok {
		return nil, ErrRootNotFound
	}
	if hasParentTraversal(path) {
		return nil, ErrPathForbidden
	}

	relativePath := CleanPath(path)
	joinedPath := filepath.Join(r.RealPath, strings.TrimPrefix(relativePath, "/"))
	absPath, err := filepath.EvalSymlinks(joinedPath)
	if err != nil {
		return nil, err
	}
	absPath = filepath.Clean(absPath)
	if !isWithinRoot(r.RealPath, absPath) {
		return nil, ErrPathForbidden
	}

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
	extensionType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	detectedType, err := detectContentType(path)
	if err == nil {
		detectedFileType := typeForMIME(detectedType)
		extensionFileType := typeForMIME(extensionType)
		if (detectedFileType == TypeImage || detectedFileType == TypeVideo) &&
			detectedFileType != extensionFileType {
			return detectedType
		}
		if extensionType == "" {
			return detectedType
		}
	} else if extensionType == "" {
		slog.Warn("failed to detect mime type", "path", path, "err", err)
	}
	if extensionType != "" {
		return extensionType
	}

	return "application/octet-stream"
}

func CleanPath(p string) string {
	if p == "" {
		return "/"
	}
	cleaned := pathpkg.Clean("/" + p)

	return cleaned
}

func TypeForFile(filePath string) string {
	extensionType := typeForExtension(filePath)
	if extensionType == TypeImage || extensionType == TypeVideo {
		return extensionType
	}

	if contentType, err := detectContentType(filePath); err == nil {
		detectedType := typeForMIME(contentType)
		if detectedType == TypeImage || detectedType == TypeVideo {
			return detectedType
		}
		if extensionType == TypeOther && filepath.Ext(filePath) == "" && detectedType == TypeText {
			return TypeText
		}
	}

	return extensionType
}

func typeForExtension(filePath string) string {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".avif":
		return TypeImage
	case ".txt", ".md", ".nfo", ".json", ".yaml", ".yml", ".xml", ".srt", ".ass", ".log":
		return TypeText
	case ".mp4", ".m4v", ".mov", ".mkv", ".webm", ".avi", ".m2ts", ".ts", ".flv", ".wmv":
		return TypeVideo
	default:
		return TypeOther
	}
}

func typeForMIME(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
	}
	switch {
	case strings.HasPrefix(mediaType, "image/"):
		return TypeImage
	case strings.HasPrefix(mediaType, "video/"):
		return TypeVideo
	case strings.HasPrefix(mediaType, "text/"):
		return TypeText
	case mediaType == "application/json",
		mediaType == "application/xml",
		mediaType == "application/yaml",
		mediaType == "application/x-yaml":
		return TypeText
	default:
		return TypeOther
	}
}

func detectContentType(filePath string) (string, error) {
	mtype, err := mimetype.DetectFile(filePath)
	if err != nil {
		return "", err
	}
	return mtype.String(), nil
}

func isWithinRoot(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func hasParentTraversal(requestPath string) bool {
	for _, part := range strings.Split(strings.ReplaceAll(requestPath, "\\", "/"), "/") {
		if part == ".." {
			return true
		}
	}
	return false
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
