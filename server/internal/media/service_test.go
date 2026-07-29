package media

import (
	"testing"

	"github.com/krteke/River/internal/config"
)

func TestPlaybackModeDirectsLowBitrateAV1(t *testing.T) {
	service := NewService("ffprobe", "ffmpeg", config.PlaybackConfig{
		DirectPlayMaxBitrate: 12_000_000,
	})
	info := playableMediaInfo()
	info.Container.Name = "matroska,webm"
	info.Container.BitRate = 4_000_000
	info.Tracks.Video[0].Codec = "av1"
	info.Tracks.Video[0].Width = 3840
	info.Tracks.Video[0].Height = 2160
	info.Tracks.Audio[0].Codec = "opus"

	if got := service.PlaybackMode(&info); got != PlaybackModeDirect {
		t.Fatalf("expected direct play, got %q", got)
	}
}

func TestPlaybackModeUsesEffectiveBitrate(t *testing.T) {
	service := NewService("ffprobe", "ffmpeg", config.PlaybackConfig{
		DirectPlayMaxBitrate: 12_000_000,
	})

	tests := map[string]struct {
		change func(*MediaInfo)
		want   PlaybackMode
	}{
		"at threshold": {
			change: func(info *MediaInfo) { info.Container.BitRate = 12_000_000 },
			want:   PlaybackModeDirect,
		},
		"above threshold": {
			change: func(info *MediaInfo) { info.Container.BitRate = 12_000_001 },
			want:   PlaybackModeTranscode,
		},
		"estimated below threshold": {
			change: func(info *MediaInfo) {
				info.Container.BitRate = 0
				info.Container.Size = 45_000_000
				info.Container.Duration = 60
			},
			want: PlaybackModeDirect,
		},
		"estimated above threshold": {
			change: func(info *MediaInfo) {
				info.Container.BitRate = 0
				info.Container.Size = 100_000_000
				info.Container.Duration = 60
			},
			want: PlaybackModeTranscode,
		},
		"unknown bitrate": {
			change: func(info *MediaInfo) {
				info.Container.BitRate = 0
				info.Container.Size = 0
				info.Container.Duration = 0
			},
			want: PlaybackModeDirect,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			info := playableMediaInfo()
			info.Tracks.Video[0].Codec = "av1"
			test.change(&info)
			if got := service.PlaybackMode(&info); got != test.want {
				t.Fatalf("unexpected playback mode: got %q want %q", got, test.want)
			}
		})
	}
}

func TestPlaybackModeRejectsMediaWithoutVideo(t *testing.T) {
	service := NewService("ffprobe", "ffmpeg", config.PlaybackConfig{})
	info := playableMediaInfo()
	info.Tracks.Video = nil

	if got := service.PlaybackMode(&info); got != PlaybackModeUnsupported {
		t.Fatalf("expected unsupported, got %q", got)
	}
}

func TestNormalizeClassifiesTextAndBitmapSubtitles(t *testing.T) {
	info := normalize(ffprobeOutput{Streams: []ffprobeStream{
		{Index: 2, CodecName: "subrip", CodecType: "subtitle"},
		{Index: 3, CodecName: "hdmv_pgs_subtitle", CodecType: "subtitle"},
	}})

	if len(info.Tracks.Subtitle) != 2 {
		t.Fatalf("unexpected subtitle tracks: %+v", info.Tracks.Subtitle)
	}
	if !info.Tracks.Subtitle[0].Text {
		t.Fatal("expected SubRip to be a text subtitle")
	}
	if info.Tracks.Subtitle[1].Text {
		t.Fatal("expected PGS to be a bitmap subtitle")
	}
}

func playableMediaInfo() MediaInfo {
	return MediaInfo{
		Container: ContainerInfo{
			Name:    "mov,mp4,m4a,3gp,3g2,mj2",
			BitRate: 8_000_000,
		},
		Tracks: TracksInfo{
			Video: []VideoTrack{{
				TrackBase: TrackBase{Codec: "h264"},
				Width:     1920,
				Height:    1080,
			}},
			Audio: []AudioTrack{{
				TrackBase: TrackBase{Codec: "aac"},
			}},
		},
	}
}
