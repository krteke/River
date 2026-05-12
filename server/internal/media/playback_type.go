package media

type PlaybackMode string

const (
	PlaybackModeDirect      PlaybackMode = "direct"
	PlaybackModeTranscode   PlaybackMode = "transcode"
	PlaybackModeUnsupported PlaybackMode = "unsupported"
)

type DirectPlayInfo struct {
	Mime string `json:"mime,omitempty"`
}

type UnsupportedInfo struct {
	Message string `json:"message,omitempty"`
}

type TranscodeInfo struct {
	TargetContainer string   `json:"target_container"`
	VideoCodec      string   `json:"video_codec,omitempty"`
	AudioCodec      string   `json:"audio_codec,omitempty"`
	Operations      []string `json:"operations,omitempty"`
}

type PlaybackInfo struct {
	Mode        PlaybackMode     `json:"mode"`
	Direct      *DirectPlayInfo  `json:"direct,omitempty"`
	TransCode   *TranscodeInfo   `json:"transcode,omitempty"`
	Unsupported *UnsupportedInfo `json:"unsupported,omitempty"`
}
