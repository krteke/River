package media

import (
	"testing"

	"github.com/krteke/River/internal/config"
)

func TestPlaybackInfoDirectPlay(t *testing.T) {
	service := NewService("ffprobe", "ffmpeg", config.PlaybackConfig{
		DirectPlayMaxBitrate: 12_000_000,
		DirectPlayMaxWidth:   1920,
		DirectPlayMaxHeight:  1080,
	})
	info := playableMediaInfo()

	playback := service.PlaybackInfo(&info)
	if playback.Mode != PlaybackModeDirect {
		t.Fatalf("expected direct play, got %q", playback.Mode)
	}
	if playback.Direct == nil || playback.Direct.Mime != "video/mp4" {
		t.Fatalf("unexpected direct play info: %+v", playback.Direct)
	}
}

func TestPlaybackInfoTranscodesIncompatibleMedia(t *testing.T) {
	service := NewService("ffprobe", "ffmpeg", config.PlaybackConfig{
		DirectPlayMaxBitrate: 12_000_000,
		DirectPlayMaxWidth:   1920,
		DirectPlayMaxHeight:  1080,
	})

	tests := map[string]func(*MediaInfo){
		"container":  func(info *MediaInfo) { info.Container.Name = "matroska,webm" },
		"codec":      func(info *MediaInfo) { info.Tracks.Video[0].Codec = "hevc" },
		"width":      func(info *MediaInfo) { info.Tracks.Video[0].Width = 3840 },
		"height":     func(info *MediaInfo) { info.Tracks.Video[0].Height = 2160 },
		"audio":      func(info *MediaInfo) { info.Tracks.Audio[0].Codec = "opus" },
		"bitrate":    func(info *MediaInfo) { info.Container.BitRate = 13_000_000 },
		"unknownBit": func(info *MediaInfo) { info.Container.BitRate = 0 },
	}

	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			info := playableMediaInfo()
			change(&info)
			if got := service.PlaybackInfo(&info).Mode; got != PlaybackModeTranscode {
				t.Fatalf("expected transcode, got %q", got)
			}
		})
	}
}

func TestPlaybackInfoRejectsMediaWithoutVideo(t *testing.T) {
	service := NewService("ffprobe", "ffmpeg", config.PlaybackConfig{})
	info := playableMediaInfo()
	info.Tracks.Video = nil

	if got := service.PlaybackInfo(&info).Mode; got != PlaybackModeUnsupported {
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
