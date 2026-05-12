package transcode

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/krteke/River/internal/config"
)

var (
	ErrQueueFull       = errors.New("transcode queue full")
	ErrSessionNotFound = errors.New("stream session not found")
	ErrProfileNotFound = errors.New("profile not found")
)

type StartOptions struct {
	RootID           string
	RelPath          string
	SourcePath       string
	ProfileName      string
	StartSeconds     float64
	ReplaceSessionID string
}

type StreamSession struct {
	ID           string    `json:"id"`
	RootID       string    `json:"root_id"`
	RelPath      string    `json:"rel_path"`
	ProfileName  string    `json:"profile_name"`
	StartSeconds float64   `json:"start_seconds"`
	TempDir      string    `json:"temp_dir"`
	StartedAt    time.Time `json:"started_at"`
	LastAccessAt time.Time `json:"last_access_at"`
	Status       string    `json:"status"`

	cmd    *exec.Cmd
	cancel context.CancelFunc
	stderr *tailBuffer
	done   chan struct{}
}

type tailBuffer struct {
	mu    sync.Mutex
	limit int
	buf   []byte
}

type Manager struct {
	cfg      *config.Config
	profiles map[string]config.ProfileConfig
	mu       sync.Mutex
	sessions map[string]*StreamSession
}

func NewManager(cfg *config.Config) *Manager {
	profiles := make(map[string]config.ProfileConfig, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		profiles[profile.Name] = profile
	}

	return &Manager{
		cfg:      cfg,
		profiles: profiles,
		sessions: make(map[string]*StreamSession),
	}
}

func (m *Manager) Start(ctx context.Context, options StartOptions) (*StreamSession, error) {
	if options.ReplaceSessionID != "" {
		m.Stop(options.ReplaceSessionID)
	}

	if options.ProfileName == "" {
		options.ProfileName = m.cfg.Playback.DefaultProfile
	}
	profile, ok := m.profiles[options.ProfileName]
	if !ok {
		return nil, ErrProfileNotFound
	}

	m.mu.Lock()
	if m.runningLocked() >= m.cfg.FFmpeg.MaxConcurrentJobs {
		m.mu.Unlock()
		return nil, ErrQueueFull
	}
	sessionID := newSessionID()
	tempDir := filepath.Join(m.cfg.Transcode.TempDir, sessionID)
	now := time.Now()
	session := &StreamSession{
		ID:           sessionID,
		RootID:       options.RootID,
		RelPath:      options.RelPath,
		ProfileName:  profile.Name,
		StartSeconds: options.StartSeconds,
		TempDir:      tempDir,
		StartedAt:    now,
		LastAccessAt: now,
		Status:       "starting",
		stderr:       newTailBuffer(64 * 1024),
		done:         make(chan struct{}),
	}
	m.sessions[sessionID] = session
	m.mu.Unlock()

	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		m.removeSession(sessionID)
		return nil, err
	}
	if err := writeMasterPlaylist(filepath.Join(tempDir, "master.m3u8"), profile); err != nil {
		m.removeSession(sessionID)
		return nil, err
	}

	// FFmpeg 不能绑定到 HTTP 请求的 context，否则客户端拿到播放地址后请求结束会中断转码。
	processCtx, cancel := context.WithCancel(context.Background())
	session.cancel = cancel
	args := buildFFmpegArgs(m.cfg, profile, options.SourcePath, tempDir, options.StartSeconds)
	cmd := exec.CommandContext(processCtx, m.cfg.FFmpeg.FFmpegPath, args...)
	cmd.Stderr = session.stderr
	session.cmd = cmd
	if err := cmd.Start(); err != nil {
		cancel()
		m.removeSession(sessionID)
		_ = os.RemoveAll(tempDir)
		return nil, err
	}

	m.mu.Lock()
	session.Status = "running"
	m.mu.Unlock()

	go m.wait(session)
	slog.InfoContext(ctx, "start transcode", "session_id", session.ID, "source", options.SourcePath, "profile", profile.Name, "start_seconds", options.StartSeconds)
	return session, nil
}

func (m *Manager) StartCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				m.StopAll()
				return
			case <-ticker.C:
				m.cleanupIdle()
			}
		}
	}()
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	sessions := make([]*StreamSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.sessions = make(map[string]*StreamSession)
	m.mu.Unlock()
	for _, session := range sessions {
		m.stopSession(session)
	}
}

func (m *Manager) stopSession(session *StreamSession) {
	if session.cancel != nil {
		session.cancel()
	}
	done := make(chan struct{})
	go func() {
		if session.done != nil {
			<-session.done
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		if session.cmd != nil && session.cmd.Process != nil {
			_ = session.cmd.Process.Kill()
		}
	}
	_ = os.RemoveAll(session.TempDir)
}

func (m *Manager) cleanupIdle() {
	deadline := time.Now().Add(-m.cfg.IdleTimeout())
	var expired []*StreamSession
	m.mu.Lock()
	for id, session := range m.sessions {
		if session.LastAccessAt.Before(deadline) {
			expired = append(expired, session)
			delete(m.sessions, id)
		}
	}
	m.mu.Unlock()
	for _, session := range expired {
		slog.Info("stop idle transcode", "session_id", session.ID)
		m.stopSession(session)
	}
}

func (m *Manager) Stop(id string) {
	m.mu.Lock()
	session, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if ok {
		m.stopSession(session)
	}
}

func (m *Manager) runningLocked() int {
	count := 0
	for _, session := range m.sessions {
		if session.Status == "starting" || session.Status == "running" {
			count++
		}
	}
	return count
}

func newSessionID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("s_%d", time.Now().UnixNano())
	}
	return "s_" + hex.EncodeToString(b[:])
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{limit: limit}
}

func (m *Manager) removeSession(id string) {
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
}

func writeMasterPlaylist(path string, profile config.ProfileConfig) error {
	width := profile.Width
	if width <= 0 {
		width = 1920
	}
	bandwidth := estimateBandwidth(profile.VideoBitrate, profile.AudioBitrate)
	content := fmt.Sprintf("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx1080\nindex.m3u8\n", bandwidth, width)
	return os.WriteFile(path, []byte(content), 0o644)
}

func buildFFmpegArgs(cfg *config.Config, profile config.ProfileConfig, sourcePath, tempDir string, startSeconds float64) []string {
	segmentDuration := profile.SegmentDuration
	if segmentDuration <= 0 {
		segmentDuration = cfg.Transcode.SegmentDurationSeconds
	}
	width := profile.Width
	if width <= 0 {
		width = cfg.Playback.DirectPlayMaxWidth
	}
	preset := profile.Preset
	if preset == "" {
		preset = "veryfast"
	}
	audioBitrate := profile.AudioBitrate
	if audioBitrate == "" {
		audioBitrate = "160k"
	}
	videoBitrate := profile.VideoBitrate
	if videoBitrate == "" {
		videoBitrate = "8000k"
	}
	audioChannels := profile.AudioChannels
	if audioChannels <= 0 {
		audioChannels = 2
	}
	args := []string{
		"-hide_banner",
		"-y",
	}
	if startSeconds > 0 {
		args = append(args, "-ss", formatStartSeconds(startSeconds))
	}
	args = append(args,
		"-i", sourcePath,
		"-map", "0:v:0?",
		"-map", "0:a:0?",
		"-vf", fmt.Sprintf("scale='min(%d,iw)':-2", width),
		"-c:v", valueOrDefault(profile.VideoCodec, "libx264"),
		"-preset", preset,
		"-b:v", videoBitrate,
		"-maxrate", videoBitrate,
		"-bufsize", bitrateBufSize(videoBitrate),
		"-profile:v", "high",
		"-pix_fmt", "yuv420p",
		"-c:a", valueOrDefault(profile.AudioCodec, "aac"),
		"-b:a", audioBitrate,
		"-ac", fmt.Sprintf("%d", audioChannels),
		"-f", "hls",
		"-hls_time", fmt.Sprintf("%d", segmentDuration),
		"-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(tempDir, "seg_%06d.ts"),
		filepath.Join(tempDir, "index.m3u8"),
	)
	return args
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if len(b.buf) > b.limit {
		b.buf = bytes.Clone(b.buf[len(b.buf)-b.limit:])
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

func (m *Manager) wait(session *StreamSession) {
	defer close(session.done)
	err := session.cmd.Wait()
	m.mu.Lock()
	if current, ok := m.sessions[session.ID]; ok {
		if err != nil {
			current.Status = "failed"
			slog.Error("transcode failed", "session_id", session.ID, "error", err, "stderr", session.stderr.String())
		} else {
			current.Status = "exited"
		}
	}
	m.mu.Unlock()
}

func estimateBandwidth(videoBitrate, audioBitrate string) int {
	return parseBitrateK(videoBitrate)*1000 + parseBitrateK(audioBitrate)*1000
}

func parseBitrateK(value string) int {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimSuffix(value, "k")
	var n int
	_, _ = fmt.Sscanf(value, "%d", &n)
	return n
}

func formatStartSeconds(startSeconds float64) string {
	return strconv.FormatFloat(startSeconds, 'f', 3, 64)
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func bitrateBufSize(videoBitrate string) string {
	n := parseBitrateK(videoBitrate)
	if n <= 0 {
		return "16000k"
	}
	return fmt.Sprintf("%dk", n*2)
}
