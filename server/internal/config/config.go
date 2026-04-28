package config

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// type SymlinkPolicy string

// const (
// 	SymlinkPolicyDeny          SymlinkPolicy = "deny"
// 	SymlinkPolicyAllowExternal SymlinkPolicy = "allow_external"
// 	SymlinkPolicyWithinRoot    SymlinkPolicy = "within_root"
// )

type Config struct {
	Server    ServerConfig    `toml:"server"`
	Roots     []RootConfig    `toml:"roots"`
	FFmpeg    FFmpegConfig    `toml:"ffmpeg"`
	Transcode TranscodeConfig `toml:"transcode"`
	Playback  PlaybackConfig  `toml:"playback"`
	Profiles  []ProfileConfig `toml:"profiles"`
}

type ServerConfig struct {
	Listen string `toml:"listen"`
}

type RootConfig struct {
	ID   string `toml:"id"`
	Name string `toml:"name"`
	Path string `toml:"path"`
	// SymlinkPolicy string `toml:"symlink_policy"`
}

type FFmpegConfig struct {
	FFmpegPath        string `toml:"ffmpeg_path"`
	FFprobePath       string `toml:"ffprobe_path"`
	MaxConcurrentJobs int    `toml:"max_concurrent_jobs"`
}

type TranscodeConfig struct {
	TempDir                string `toml:"temp_dir"`
	IdleTimeoutSeconds     int    `toml:"idle_timeout_seconds"`
	SegmentDurationSeconds int    `toml:"segment_duration_seconds"`
}

type PlaybackConfig struct {
	DirectPlayMaxBitrate int64  `toml:"direct_play_max_bitrate"`
	DirectPlayMaxWidth   int    `toml:"direct_play_max_width"`
	DirectPlayMaxHeight  int    `toml:"direct_play_max_height"`
	DefaultProfile       string `toml:"default_profile"`
}

type ProfileConfig struct {
	Name            string `toml:"name"`
	Container       string `toml:"container"`
	VideoCodec      string `toml:"video_codec"`
	Width           int    `toml:"width"`
	VideoBitrate    string `toml:"video_bitrate"`
	AudioCodec      string `toml:"audio_codec"`
	AudioBitrate    string `toml:"audio_bitrate"`
	AudioChannels   int    `toml:"audio_channels"`
	Preset          string `toml:"preset"`
	SegmentDuration int    `toml:"segment_duration"`
}

func (c Config) IdleTimeout() time.Duration {
	return time.Duration(c.Transcode.IdleTimeoutSeconds) * time.Second
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Listen: "0.0.0.0:8080",
		},
		FFmpeg: FFmpegConfig{
			FFmpegPath:        "/usr/bin/ffmpeg",
			FFprobePath:       "/usr/bin/ffprobe",
			MaxConcurrentJobs: 3,
		},
		Transcode: TranscodeConfig{
			TempDir:                filepath.Join(os.TempDir(), "/temp-transcode"),
			IdleTimeoutSeconds:     60,
			SegmentDurationSeconds: 6,
		},
		Playback: PlaybackConfig{
			DirectPlayMaxBitrate: 8_000_000,
			DirectPlayMaxWidth:   1920,
			DirectPlayMaxHeight:  1080,
			DefaultProfile:       "1080p_8m",
		},
		Profiles: []ProfileConfig{
			{
				Name:            "1080p_8m",
				Container:       "hls_ts",
				VideoCodec:      "libx264",
				Width:           1920,
				VideoBitrate:    "8000k",
				AudioCodec:      "aac",
				AudioBitrate:    "160k",
				AudioChannels:   2,
				Preset:          "veryfast",
				SegmentDuration: 6,
			},
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("config file not found at " + path)
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, errors.New("failed to decode config file: " + err.Error())
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c Config) Validate() error {
	if len(c.Roots) == 0 {
		return errors.New("at least one root is required")
	}
	if c.Server.Listen == "" {
		return errors.New("server.listen is required")
	}
	if c.FFmpeg.FFmpegPath == "" {
		return errors.New("ffmpeg.ffmpeg_path is required")
	}
	if c.FFmpeg.FFprobePath == "" {
		return errors.New("ffmpeg.ffprobe_path is required")
	}
	if c.FFmpeg.MaxConcurrentJobs <= 0 {
		return errors.New("ffmpeg.max_concurrent_jobs must be greater than 0")
	}
	if c.Transcode.TempDir == "" {
		return errors.New("transcode.temp_dir is required")
	}
	if c.Transcode.IdleTimeoutSeconds <= 0 {
		return errors.New("transcode.idle_timeout_seconds must be greater than 0")
	}
	if c.Transcode.SegmentDurationSeconds <= 0 {
		return errors.New("transcode.segment_duration_seconds must be greater than 0")
	}
	if c.Playback.DirectPlayMaxBitrate <= 0 {
		return errors.New("playback.direct_play_max_bitrate must be greater than 0")
	}
	if c.Playback.DirectPlayMaxWidth <= 0 {
		return errors.New("playback.direct_play_max_width must be greater than 0")
	}
	if c.Playback.DirectPlayMaxHeight <= 0 {
		return errors.New("playback.direct_play_max_height must be greater than 0")
	}
	if c.Playback.DefaultProfile == "" {
		return errors.New("playback.default_profile is required")
	}
	if len(c.Profiles) == 0 {
		return errors.New("at least one profile is required")
	}

	seenRoots := make(map[string]struct{}, len(c.Roots))
	for _, root := range c.Roots {
		if root.ID == "" {
			return errors.New("root.id is required")
		}
		if root.Path == "" {
			return errors.New("root.path is required")
		}
		if _, ok := seenRoots[root.ID]; ok {
			return errors.New("duplicate root id: " + root.ID)
		}
		seenRoots[root.ID] = struct{}{}
	}

	profiles := make(map[string]struct{}, len(c.Profiles))
	for _, profile := range c.Profiles {
		if profile.Name == "" {
			return errors.New("profile.name is required")
		}
		if _, ok := profiles[profile.Name]; ok {
			return errors.New("duplicate profile name: " + profile.Name)
		}
		if profile.Container == "" {
			return errors.New("profile.container is required")
		}
		if profile.VideoCodec == "" {
			return errors.New("profile.video_codec is required")
		}
		if profile.AudioCodec == "" {
			return errors.New("profile.audio_codec is required")
		}
		if profile.Width <= 0 {
			return errors.New("profile.width must be greater than 0")
		}
		if profile.VideoBitrate == "" {
			return errors.New("profile.video_bitrate is required")
		}
		if profile.AudioBitrate == "" {
			return errors.New("profile.audio_bitrate is required")
		}
		if profile.AudioChannels <= 0 {
			return errors.New("profile.audio_channels must be greater than 0")
		}
		if profile.Preset == "" {
			return errors.New("profile.preset is required")
		}
		if profile.SegmentDuration <= 0 {
			return errors.New("profile.segment_duration must be greater than 0")
		}
		profiles[profile.Name] = struct{}{}
	}

	return nil
}
