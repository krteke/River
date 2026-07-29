package media

type PlaybackMode string

const (
	PlaybackModeDirect      PlaybackMode = "direct"
	PlaybackModeTranscode   PlaybackMode = "transcode"
	PlaybackModeUnsupported PlaybackMode = "unsupported"
)
