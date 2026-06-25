package thumbnail

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/krteke/River/internal/config"
	filesystem "github.com/krteke/River/internal/fs"
)

const (
	contentTypeJPEG = "image/jpeg"
	commandTimeout  = 30 * time.Second
)

var ErrFFmpegNotAvailable = errors.New("ffmpeg not available")

type Service struct {
	ffmpegPath      string
	cacheDir        string
	width           int
	height          int
	videoSeek       time.Duration
	maxAge          time.Duration
	maxSizeBytes    int64
	cleanupInterval time.Duration

	mu          sync.Mutex
	lastCleanup time.Time
}

type Result struct {
	Path        string
	ContentType string
	ModTime     time.Time
}

func NewService(ffmpegPath string, cfg config.ThumbnailConfig) *Service {
	return &Service{
		ffmpegPath:      ffmpegPath,
		cacheDir:        cfg.CacheDir,
		width:           cfg.Width,
		height:          cfg.Height,
		videoSeek:       time.Duration(cfg.VideoSeekSeconds) * time.Second,
		maxAge:          time.Duration(cfg.MaxAgeSeconds) * time.Second,
		maxSizeBytes:    cfg.MaxSizeMB * 1024 * 1024,
		cleanupInterval: time.Duration(cfg.CleanupIntervalSeconds) * time.Second,
	}
}

func (s *Service) Get(ctx context.Context, source *filesystem.ResolvedPath) (*Result, error) {
	if err := os.MkdirAll(s.cacheDir, 0o755); err != nil {
		return nil, err
	}

	cachePath := filepath.Join(s.cacheDir, s.cacheName(source))
	if info, err := os.Stat(cachePath); err == nil && info.Size() > 0 {
		_ = os.Chtimes(cachePath, time.Now(), info.ModTime())
		return &Result{Path: cachePath, ContentType: contentTypeJPEG, ModTime: info.ModTime()}, nil
	}

	tmp, err := os.CreateTemp(s.cacheDir, "thumb-*.jpg")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)

	if err := s.generate(ctx, source, tmpPath); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		return nil, err
	}

	s.cleanupIfDue()

	info, err := os.Stat(cachePath)
	if err != nil {
		return nil, err
	}

	return &Result{Path: cachePath, ContentType: contentTypeJPEG, ModTime: info.ModTime()}, nil
}

func (s *Service) Cleanup() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastCleanup = time.Now()
	return s.cleanupLocked()
}

func (s *Service) cleanupIfDue() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.lastCleanup) < s.cleanupInterval {
		return
	}
	s.lastCleanup = time.Now()
	_ = s.cleanupLocked()
}

func (s *Service) cleanupLocked() error {
	entries, err := os.ReadDir(s.cacheDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	now := time.Now()
	files := make([]cacheFile, 0, len(entries))
	var total int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(s.cacheDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > s.maxAge {
			_ = os.Remove(path)
			continue
		}
		total += info.Size()
		files = append(files, cacheFile{
			path:    path,
			size:    info.Size(),
			modTime: info.ModTime(),
		})
	}
	if total <= s.maxSizeBytes {
		return nil
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})
	for _, file := range files {
		if total <= s.maxSizeBytes {
			break
		}
		if err := os.Remove(file.path); err == nil {
			total -= file.size
		}
	}

	return nil
}

func (s *Service) generate(ctx context.Context, source *filesystem.ResolvedPath, outputPath string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	args := []string{"-hide_banner", "-loglevel", "error", "-y"}
	sourceType := filesystem.TypeForFile(source.AbsPath)
	if sourceType == filesystem.TypeVideo && s.videoSeek > 0 {
		args = append(args, "-ss", formatSeconds(s.videoSeek))
	}
	filter := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", s.width, s.height)
	if sourceType == filesystem.TypeVideo {
		filter = "thumbnail," + filter
	}
	args = append(args,
		"-i", source.AbsPath,
		"-frames:v", "1",
		"-vf", filter,
		"-q:v", "3",
		outputPath,
	)

	output, err := exec.CommandContext(timeoutCtx, s.ffmpegPath, args...).CombinedOutput()
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("%w: %v", ErrFFmpegNotAvailable, err)
	}
	if timeoutCtx.Err() != nil {
		return fmt.Errorf("thumbnail generation timed out for %q: %w", filepath.Base(source.AbsPath), timeoutCtx.Err())
	}
	if err != nil {
		return fmt.Errorf("thumbnail generation failed for %q: %w: %s", filepath.Base(source.AbsPath), err, string(output))
	}

	return nil
}

func (s *Service) cacheName(source *filesystem.ResolvedPath) string {
	hash := sha256.Sum256([]byte(stringsJoin(
		source.Root.ID,
		source.RelPath,
		source.AbsPath,
		strconv.FormatInt(source.Info.Size(), 10),
		strconv.FormatInt(source.Info.ModTime().UnixNano(), 10),
		strconv.Itoa(s.width),
		strconv.Itoa(s.height),
		strconv.FormatInt(int64(s.videoSeek/time.Second), 10),
	)))
	return hex.EncodeToString(hash[:]) + ".jpg"
}

func formatSeconds(duration time.Duration) string {
	return strconv.FormatFloat(duration.Seconds(), 'f', 3, 64)
}

func stringsJoin(values ...string) string {
	var total int
	for _, value := range values {
		total += len(value) + 1
	}
	joined := make([]byte, 0, total)
	for _, value := range values {
		joined = append(joined, value...)
		joined = append(joined, 0)
	}
	return string(joined)
}

type cacheFile struct {
	path    string
	size    int64
	modTime time.Time
}
