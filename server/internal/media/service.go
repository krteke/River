package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/krteke/River/internal/config"
)

const defaultProbeTimeout = 30 * time.Second
const defaultSubtitleTimeout = 30 * time.Second
const maxSubtitleOutputSize = 10 << 20

var ErrFFprobeNotAvailable = errors.New("ffprobe not available")
var ErrFFmpegNotAvailable = errors.New("ffmpeg not available")
var ErrUnsupportedSubtitle = errors.New("subtitle format is not supported")
var ErrSubtitleTooLarge = errors.New("subtitle is too large")

type Service struct {
	ffprobePath string
	ffmpegPath  string
	playback    config.PlaybackConfig
}

func NewService(ffprobePath, ffmpegPath string, playback config.PlaybackConfig) *Service {
	return &Service{ffprobePath: ffprobePath, ffmpegPath: ffmpegPath, playback: playback}
}

func (s *Service) Probe(ctx context.Context, filePath string) (*MediaInfo, error) {
	raw, err := s.probeRaw(ctx, filePath)
	if err != nil {
		return nil, err
	}

	info := normalize(raw)

	return &info, nil
}

// ExtractSubtitle converts a text subtitle stream to WebVTT for the client player.
// Bitmap subtitles require video rendering and are intentionally not converted here.
func (s *Service) ExtractSubtitle(ctx context.Context, filePath string, track SubtitleTrack) ([]byte, error) {
	if !track.Text {
		return nil, ErrUnsupportedSubtitle
	}

	ctx, cancel := context.WithTimeout(ctx, defaultSubtitleTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.ffmpegPath,
		"-hide_banner",
		"-v", "error",
		"-nostdin",
		"-i", filePath,
		"-map", fmt.Sprintf("0:%d", track.Index),
		"-c:s", "webvtt",
		"-f", "webvtt",
		"pipe:1",
	)

	var stdout limitedBuffer
	stdout.limit = maxSubtitleOutputSize
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stdout.exceeded {
			return nil, ErrSubtitleTooLarge
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("subtitle extraction timeout after %s for %q: %w", defaultSubtitleTimeout, filepath.Base(filePath), ctx.Err())
		}
		var execErr *exec.Error
		if errors.As(err, &execErr) || errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %v", ErrFFmpegNotAvailable, err)
		}
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, fmt.Errorf("extract subtitle from %q: %w: %s", filepath.Base(filePath), err, errMsg)
	}
	if stdout.exceeded {
		return nil, ErrSubtitleTooLarge
	}
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("subtitle extraction returned empty output for %q", filepath.Base(filePath))
	}

	return stdout.Bytes(), nil
}

func (s *Service) probeRaw(ctx context.Context, filePath string) (ffprobeOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultProbeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.ffprobePath,
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ffprobeOutput{}, fmt.Errorf("ffprobe timeout after %s for %q: %w", defaultProbeTimeout, filepath.Base(filePath), ctx.Err())
		}
		var execErr *exec.Error
		if errors.As(err, &execErr) || errors.Is(err, os.ErrNotExist) {
			return ffprobeOutput{}, fmt.Errorf("%w: %v", ErrFFprobeNotAvailable, err)
		}

		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}

		return ffprobeOutput{}, fmt.Errorf("ffprobe error for %q: %w: %s", filepath.Base(filePath), err, errMsg)
	}

	data := bytes.TrimSpace(stdout.Bytes())
	if len(data) == 0 {
		return ffprobeOutput{}, fmt.Errorf("ffprobe returned empty output for %q", filepath.Base(filePath))
	}

	var raw ffprobeOutput
	if err := json.Unmarshal(data, &raw); err != nil {
		return ffprobeOutput{}, fmt.Errorf("parse ffprobe json for %q: %w", filepath.Base(filePath), err)
	}

	return raw, nil
}

func (s *Service) PlaybackMode(info *MediaInfo) PlaybackMode {
	if len(info.Tracks.Video) == 0 {
		return PlaybackModeUnsupported
	}

	bitRate := effectiveBitRate(info)
	if bitRate <= 0 || bitRate <= float64(s.playback.DirectPlayMaxBitrate) {
		return PlaybackModeDirect
	}

	return PlaybackModeTranscode
}

func effectiveBitRate(info *MediaInfo) float64 {
	if info.Container.BitRate > 0 {
		return float64(info.Container.BitRate)
	}
	if info.Container.Size <= 0 || info.Container.Duration <= 0 {
		return 0
	}
	return float64(info.Container.Size) * 8 / info.Container.Duration
}

func normalize(raw ffprobeOutput) MediaInfo {
	media := MediaInfo{
		Container: ContainerInfo{
			Name:      raw.Format.FormatName,
			Duration:  parseFloat(raw.Format.Duration),
			BitRate:   parseInt64(raw.Format.BitRate),
			Size:      parseInt64(raw.Format.Size),
			StartTime: parseFloat(raw.Format.StartTime),
		},
	}

	for _, stream := range raw.Streams {
		base := trackBaseFromFFProbe(stream)

		switch strings.ToLower(stream.CodecType) {
		case "video":
			track := VideoTrack{
				TrackBase:      base,
				Width:          stream.Width,
				Height:         stream.Height,
				PixFmt:         stream.PixFmt,
				BitRate:        parseInt64(stream.BitRate),
				ColorRange:     stream.ColorRange,
				ColorSpace:     stream.ColorSpace,
				ColorTransfer:  stream.ColorTransfer,
				ColorPrimaries: stream.ColorPrimaries,
				AvgFrameRate:   parseRational(stream.AvgFrameRate),
				RFrameRate:     parseRational(stream.RFrameRate),
				TimeBase:       parseRational(stream.TimeBase),
				Duration:       parseFloat(stream.Duration),
			}
			media.Tracks.Video = append(media.Tracks.Video, track)
		case "audio":
			media.Tracks.Audio = append(media.Tracks.Audio, AudioTrack{
				TrackBase:     base,
				Channels:      stream.Channels,
				ChannelLayout: stream.ChannelLayout,
				SampleRate:    parseInt64(stream.SampleRate),
				BitRate:       parseInt64(stream.BitRate),
				Duration:      parseFloat(stream.Duration),
			})
		case "subtitle":
			media.Tracks.Subtitle = append(media.Tracks.Subtitle, SubtitleTrack{
				TrackBase: base,
				Text:      textSubtitleCodec(stream.CodecName),
			})
		default:
			media.Tracks.Other = append(media.Tracks.Other, base)
		}
	}

	return media
}

func textSubtitleCodec(codec string) bool {
	switch strings.ToLower(codec) {
	case "ass", "ssa", "subrip", "srt", "webvtt", "mov_text", "text", "tx3g":
		return true
	default:
		return false
	}
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		b.exceeded = true
		return len(p), nil
	}
	return b.Buffer.Write(p)
}

func trackBaseFromFFProbe(stream ffprobeStream) TrackBase {
	return TrackBase{
		Index:    stream.Index,
		Codec:    stream.CodecName,
		Profile:  stream.Profile,
		Language: tagValue(stream.Tags, "language"),
		Title:    tagValue(stream.Tags, "title"),
		Default:  dispositionValue(stream.Disposition, "default") == 1,
		Forced:   dispositionValue(stream.Disposition, "forced") == 1,
	}
}

func tagValue(tags map[string]string, key string) string {
	if tags == nil {
		return ""
	}
	return tags[key]
}

func dispositionValue(disposition map[string]int, key string) int {
	if disposition == nil {
		return 0
	}
	return disposition[key]
}

func parseFloat(value string) float64 {
	f, _ := strconv.ParseFloat(value, 64)
	return f
}

func parseInt64(value string) int64 {
	i, _ := strconv.ParseInt(value, 10, 64)
	return i
}

func parseRational(value string) *Rational {
	r := &Rational{}

	if value == "" {
		return r
	}

	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return r
	}

	r.Num = parseInt64(strings.TrimSpace(parts[0]))
	r.Den = parseInt64(strings.TrimSpace(parts[1]))

	return r
}
