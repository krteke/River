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
	ErrQueueFull          = errors.New("transcode queue full")
	ErrSessionNotFound    = errors.New("stream session not found")
	ErrProfileNotFound    = errors.New("profile not found")
	ErrInvalidStreamFile  = errors.New("invalid stream file")
	ErrFFmpegNotAvailable = errors.New("ffmpeg not available")
)

const transcodeReadyTimeout = 15 * time.Second

type transcodeMode string

const (
	transcodeModeSoftware transcodeMode = "software"
	transcodeModeHardware transcodeMode = "hardware"
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

	sourcePath string
	mode       transcodeMode
	cmd        *exec.Cmd
	cancel     context.CancelFunc
	stderr     *tailBuffer
	done       chan struct{}
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
		sourcePath:   options.SourcePath,
		stderr:       newTailBuffer(64 * 1024),
		done:         make(chan struct{}),
	}

	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return nil, err
	}
	if err := writeMasterPlaylist(filepath.Join(tempDir, "master.m3u8"), profile); err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}

	m.mu.Lock()
	if m.runningLocked() >= m.cfg.FFmpeg.MaxConcurrentJobs {
		m.mu.Unlock()
		_ = os.RemoveAll(tempDir)
		return nil, ErrQueueFull
	}
	m.sessions[sessionID] = session
	m.mu.Unlock()

	modes := m.transcodeModes()
	var lastErr error
	for i, mode := range modes {
		if i > 0 {
			m.prepareFallback(session)
		}
		if err := m.startProcess(session, mode, buildFFmpegArgs(m.cfg, profile, options.SourcePath, tempDir, options.StartSeconds, mode)); err != nil {
			lastErr = err
		} else if err := m.waitReady(ctx, session); err != nil {
			lastErr = err
			m.stopProcess(session)
		} else {
			slog.InfoContext(ctx, "start transcode", "session_id", session.ID, "source", options.SourcePath, "profile", profile.Name, "start_seconds", options.StartSeconds, "mode", mode)
			return session, nil
		}

		if mode == transcodeModeHardware && i+1 < len(modes) {
			slog.WarnContext(ctx, "hardware transcode failed, falling back to software", "session_id", session.ID, "source", options.SourcePath, "profile", profile.Name, "error", lastErr)
			continue
		}
		m.Stop(session.ID)
		return nil, lastErr
	}

	m.Stop(session.ID)
	return nil, lastErr
}

func (m *Manager) transcodeModes() []transcodeMode {
	if !m.cfg.Hardware.Enabled {
		return []transcodeMode{transcodeModeSoftware}
	}
	if m.cfg.Hardware.FallbackToSoftware {
		return []transcodeMode{transcodeModeHardware, transcodeModeSoftware}
	}
	return []transcodeMode{transcodeModeHardware}
}

func (m *Manager) startProcess(session *StreamSession, mode transcodeMode, args []string) error {
	// FFmpeg 不能绑定到 HTTP 请求的 context，否则客户端拿到播放地址后请求结束会中断转码。
	processCtx, cancel := context.WithCancel(context.Background())
	stderr := newTailBuffer(64 * 1024)
	done := make(chan struct{})
	cmd := exec.CommandContext(processCtx, m.cfg.FFmpeg.FFmpegPath, args...)
	cmd.Stderr = stderr

	m.mu.Lock()
	session.mode = mode
	session.cancel = cancel
	session.stderr = stderr
	session.done = done
	session.cmd = cmd
	session.Status = "starting"
	if err := cmd.Start(); err != nil {
		session.Status = "failed"
		m.mu.Unlock()
		cancel()
		close(done)
		var execErr *exec.Error
		if errors.As(err, &execErr) || errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %v", ErrFFmpegNotAvailable, err)
		}
		return err
	}
	session.Status = "running"
	m.mu.Unlock()

	go m.wait(session, cmd, stderr, done, mode)
	return nil
}

func (m *Manager) prepareFallback(session *StreamSession) {
	m.stopProcess(session)
	_ = cleanupHLSFiles(session.TempDir)
}

func (m *Manager) OpenStreamFile(sessionID, name string) (*os.File, error) {
	if !validStreamFile(name) {
		return nil, ErrInvalidStreamFile
	}

	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	m.mu.Unlock()
	if !ok {
		return nil, ErrSessionNotFound
	}

	file, err := os.Open(filepath.Join(session.TempDir, name))
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	if current, ok := m.sessions[sessionID]; ok && current == session {
		current.LastAccessAt = time.Now()
	}
	m.mu.Unlock()
	return file, nil
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
		slog.Info("stop transcode", "session_id", session.ID)
		m.stopSession(session)
	}
}

func (m *Manager) stopSession(session *StreamSession) {
	m.stopProcess(session)
	_ = os.RemoveAll(session.TempDir)
}

func (m *Manager) stopProcess(session *StreamSession) {
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
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	}
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

func (m *Manager) Stop(id string) bool {
	m.mu.Lock()
	session, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if ok {
		slog.Info("stop transcode", "session_id", session.ID)
		m.stopSession(session)
	}
	return ok
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
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("s_%d", time.Now().UnixNano())
	}
	return "s_" + hex.EncodeToString(b[:])
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{limit: limit}
}

func writeMasterPlaylist(path string, profile config.ProfileConfig) error {
	bandwidth := estimateBandwidth(profile.VideoBitrate, profile.AudioBitrate)
	content := fmt.Sprintf("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=%d\nindex.m3u8\n", bandwidth)
	return os.WriteFile(path, []byte(content), 0o644)
}

func buildFFmpegArgs(cfg *config.Config, profile config.ProfileConfig, sourcePath, tempDir string, startSeconds float64, mode transcodeMode) []string {
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
	if mode == transcodeModeHardware {
		args = append(args, hardwareInputArgs(cfg, profile)...)
	}
	args = append(args,
		"-i", sourcePath,
		"-map", "0:v:0?",
		"-map", "0:a:0?",
	)
	args = append(args, videoTranscodeArgs(cfg, profile, mode, width, preset)...)
	args = append(args,
		"-b:v", videoBitrate,
		"-maxrate", videoBitrate,
		"-bufsize", bitrateBufSize(videoBitrate),
		"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", segmentDuration),
	)
	if mode == transcodeModeHardware {
		args = append(args, cfg.Hardware.OutputArgs...)
	}
	if tag := videoTag(cfg, profile, mode); tag != "" {
		args = append(args, "-tag:v", tag)
	}
	args = append(args, audioTranscodeArgs(profile, audioBitrate, audioChannels)...)
	args = append(args,
		"-f", "hls",
		"-hls_time", fmt.Sprintf("%d", segmentDuration),
		"-hls_list_size", "0",
		"-hls_flags", "independent_segments",
	)
	args = append(args, hlsSegmentArgs(profile, tempDir)...)
	args = append(args, filepath.Join(tempDir, "index.m3u8"))
	return args
}

func videoTranscodeArgs(cfg *config.Config, profile config.ProfileConfig, mode transcodeMode, width int, preset string) []string {
	filter := defaultVideoFilter(width)
	if mode == transcodeModeHardware {
		codec := hardwareVideoCodec(cfg, profile)
		filter = hardwareVideoFilter(cfg, profile, width)
		args := []string{
			"-vf", filter,
			"-c:v", codec,
		}
		if videoProfile := videoProfile(profile, codec); videoProfile != "" {
			args = append(args, "-profile:v", videoProfile)
		}
		return args
	}

	codec := valueOrDefault(profile.VideoCodec, "libx264")
	args := []string{
		"-vf", filter,
		"-c:v", codec,
		"-preset", preset,
	}
	if videoProfile := videoProfile(profile, codec); videoProfile != "" {
		args = append(args, "-profile:v", videoProfile)
	}
	if pixelFormat := pixelFormat(profile, codec); pixelFormat != "" {
		args = append(args, "-pix_fmt", pixelFormat)
	}
	return args
}

func defaultVideoFilter(width int) string {
	return fmt.Sprintf("scale='min(%d,iw)':-2", width)
}

func hardwareInputArgs(cfg *config.Config, profile config.ProfileConfig) []string {
	if len(cfg.Hardware.InputArgs) > 0 {
		return cfg.Hardware.InputArgs
	}
	if isVAAPIEncoder(hardwareVideoCodec(cfg, profile)) {
		args := []string{}
		if cfg.Hardware.Device != "" {
			args = append(args, "-vaapi_device", cfg.Hardware.Device)
		}
		return append(args, "-hwaccel", "vaapi", "-hwaccel_output_format", "vaapi")
	}
	return nil
}

func hardwareVideoFilter(cfg *config.Config, profile config.ProfileConfig, width int) string {
	if cfg.Hardware.VideoFilter != "" {
		return cfg.Hardware.VideoFilter
	}
	if isVAAPIEncoder(hardwareVideoCodec(cfg, profile)) {
		return fmt.Sprintf("scale_vaapi=w=%d:h=-2:format=nv12", width)
	}
	return defaultVideoFilter(width)
}

func isVAAPIEncoder(codec string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(codec)), "_vaapi")
}

func hardwareVideoCodec(cfg *config.Config, profile config.ProfileConfig) string {
	return valueOrDefault(profile.HardwareVideoCodec, cfg.Hardware.VideoCodec)
}

func videoProfile(profile config.ProfileConfig, codec string) string {
	if profile.VideoProfile != "" {
		return profile.VideoProfile
	}
	if isHEVCEncoder(codec) {
		return "main"
	}
	if isH264Encoder(codec) {
		return "high"
	}
	return ""
}

func pixelFormat(profile config.ProfileConfig, codec string) string {
	if profile.PixelFormat != "" {
		return profile.PixelFormat
	}
	if isH264Encoder(codec) || isHEVCEncoder(codec) {
		return "yuv420p"
	}
	return ""
}

func videoTag(cfg *config.Config, profile config.ProfileConfig, mode transcodeMode) string {
	if profile.VideoTag != "" {
		return profile.VideoTag
	}
	codec := profile.VideoCodec
	if mode == transcodeModeHardware {
		codec = hardwareVideoCodec(cfg, profile)
	}
	if strings.EqualFold(profile.Container, "hls_fmp4") && isHEVCEncoder(codec) {
		return "hvc1"
	}
	return ""
}

func audioTranscodeArgs(profile config.ProfileConfig, audioBitrate string, audioChannels int) []string {
	codec := valueOrDefault(profile.AudioCodec, "aac")
	args := []string{"-c:a", codec}
	if codec == "copy" {
		return args
	}
	return append(args, "-b:a", audioBitrate, "-ac", fmt.Sprintf("%d", audioChannels))
}

func hlsSegmentArgs(profile config.ProfileConfig, tempDir string) []string {
	if strings.EqualFold(profile.Container, "hls_fmp4") {
		return []string{
			"-hls_segment_type", "fmp4",
			"-hls_fmp4_init_filename", "init.mp4",
			"-hls_segment_filename", filepath.Join(tempDir, "seg_%06d.m4s"),
		}
	}
	return []string{"-hls_segment_filename", filepath.Join(tempDir, "seg_%06d.ts")}
}

func isH264Encoder(codec string) bool {
	normalized := strings.ToLower(strings.TrimSpace(codec))
	return strings.Contains(normalized, "264") || strings.Contains(normalized, "avc")
}

func isHEVCEncoder(codec string) bool {
	normalized := strings.ToLower(strings.TrimSpace(codec))
	return strings.Contains(normalized, "265") || strings.Contains(normalized, "hevc")
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

func cleanupHLSFiles(tempDir string) error {
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "master.m3u8" {
			continue
		}
		if name == "index.m3u8" || name == "init.mp4" || strings.HasPrefix(name, "seg_") {
			if err := os.Remove(filepath.Join(tempDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) wait(session *StreamSession, cmd *exec.Cmd, stderr *tailBuffer, done chan struct{}, mode transcodeMode) {
	defer close(done)
	err := cmd.Wait()
	m.mu.Lock()
	if current, ok := m.sessions[session.ID]; ok {
		if current.cmd != cmd {
			m.mu.Unlock()
			return
		}
		if err != nil {
			current.Status = "failed"
			exitCode := -1
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				exitCode = exitErr.ExitCode()
			}
			slog.Error("transcode failed",
				"session_id", session.ID,
				"source", session.sourcePath,
				"profile", session.ProfileName,
				"mode", mode,
				"exit_code", exitCode,
				"error", err,
				"stderr", stderr.String(),
			)
		} else {
			current.Status = "exited"
		}
	}
	m.mu.Unlock()
}

func (m *Manager) waitReady(ctx context.Context, session *StreamSession) error {
	timer := time.NewTimer(transcodeReadyTimeout)
	defer timer.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	playlistPath := filepath.Join(session.TempDir, "index.m3u8")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-session.done:
			if info, err := os.Stat(playlistPath); err == nil && info.Size() > 0 {
				return nil
			}
			return fmt.Errorf("ffmpeg exited before HLS was ready: %s", strings.TrimSpace(session.stderr.String()))
		case <-timer.C:
			return errors.New("timed out waiting for HLS playlist")
		case <-ticker.C:
			if info, err := os.Stat(playlistPath); err == nil && info.Size() > 0 {
				return nil
			}
		}
	}
}

func validStreamFile(name string) bool {
	if name == "master.m3u8" || name == "index.m3u8" || name == "init.mp4" {
		return true
	}
	if strings.HasPrefix(name, "seg_") && strings.HasSuffix(name, ".m4s") && len(name) == len("seg_000000.m4s") {
		return allDigits(name[len("seg_") : len(name)-len(".m4s")])
	}
	if !strings.HasPrefix(name, "seg_") || !strings.HasSuffix(name, ".ts") || len(name) != len("seg_000000.ts") {
		return false
	}
	return allDigits(name[len("seg_") : len(name)-len(".ts")])
}

func allDigits(value string) bool {
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
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
