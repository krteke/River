package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultProbeTimeout = 30 * time.Second

type Service struct {
	ffprobePath string
}

func NewService(ffprobePath string) *Service {
	return &Service{ffprobePath: ffprobePath}
}

func (s *Service) Probe(ctx context.Context, filePath string) (*MediaInfo, error) {
	if strings.TrimSpace(filePath) == "" {
		return nil, fmt.Errorf("file path is not empty")
	}

	raw, err := s.probeRaw(ctx, filePath)
	if err != nil {
		return nil, err
	}

	info := normalize(raw)

	return &info, nil
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

		switch strings.ToLower(stream.CodecName) {
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
			})
		default:
			media.Tracks.Other = append(media.Tracks.Other, base)
		}
	}

	return media
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

// func isHDR(video VideoTrack) bool {
// 	transfer := strings.ToLower(video.ColorTransfer)
// 	primaries := strings.ToLower(video.ColorPrimaries)

// 	return transfer == "smpte2084" ||
// 		transfer == "arib-std-b67" ||
// 		primaries == "bt2020"
// }
