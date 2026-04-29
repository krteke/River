package transcode

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/krteke/River/internal/config"
)

var (
	ErrQueueFull       = errors.New("transcode queue full")
	ErrSessionNotFound = errors.New("stream session not found")
	ErrProfileNotFound = errors.New("profile not found")
)

type StreamSession struct {
}

type Manager struct {
	cfg      *config.Config
	profiles map[string]config.ProfileConfig
	logger   *slog.Logger
	mu       sync.Mutex
	sessions map[string]*StreamSession
}

func NewManager(cfg *config.Config, logger *slog.Logger) *Manager {
	profiles := make(map[string]config.ProfileConfig, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		profiles[profile.Name] = profile
	}

	return &Manager{
		cfg:      cfg,
		profiles: profiles,
		logger:   logger,
		sessions: make(map[string]*StreamSession),
	}
}

func (m *Manager) StartCleanupLoop(ctx context.Context) {
	// todo
}
