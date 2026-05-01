package transcode

import (
	"context"
	"errors"
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

func (m *Manager) StartCleanupLoop(ctx context.Context) {
	// todo
}

// func isHDR(video media.VideoTrack) bool {
// 	transfer := strings.ToLower(video.ColorTransfer)
// 	primaries := strings.ToLower(video.ColorPrimaries)

// 	return transfer == "smpte2084" ||
// 		transfer == "arib-std-b67" ||
// 		primaries == "bt2020"
// }
