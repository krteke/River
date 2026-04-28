package media

type ContainerInfo struct {
	Name      string  `json:"name"`
	Duration  float64 `json:"duration"`
	BitRate   int64   `json:"bit_rate,omitempty"`
	Size      int64   `json:"size,omitempty"`
	StartTime float64 `json:"start_time,omitempty"`
}

type Rational struct {
	Num int64 `json:"num"`
	Den int64 `json:"den"`
}

type VideoTrack struct {
	TrackBase

	Width          int       `json:"width,omitempty"`
	Height         int       `json:"height,omitempty"`
	PixFmt         string    `json:"pix_fmt,omitempty"`
	BitRate        int64     `json:"bit_rate,omitempty"`
	ColorRange     string    `json:"color_range,omitempty"`
	ColorSpace     string    `json:"color_space,omitempty"`
	ColorTransfer  string    `json:"color_transfer,omitempty"`
	ColorPrimaries string    `json:"color_primaries,omitempty"`
	AvgFrameRate   *Rational `json:"avg_frame_rate,omitempty"`
	RFrameRate     *Rational `json:"r_frame_rate,omitempty"`
	TimeBase       *Rational `json:"time_base,omitempty"`
	Duration       float64   `json:"duration,omitempty"`
	HDR            bool      `json:"hdr,omitempty"`
}

type AudioTrack struct {
	TrackBase

	Channels      int     `json:"channels,omitempty"`
	ChannelLayout string  `json:"channel_layout,omitempty"`
	SampleRate    int64   `json:"sample_rate,omitempty"`
	BitRate       int64   `json:"bit_rate,omitempty"`
	Duration      float64 `json:"duration,omitempty"`
}

type SubtitleTrack struct {
	TrackBase
}

type TrackBase struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Profile  string `json:"profile,omitempty"`
	Language string `json:"language,omitempty"`
	Title    string `json:"title,omitempty"`
	Default  bool   `json:"default,omitempty"`
	Forced   bool   `json:"forced,omitempty"`
}

type TracksInfo struct {
	Video    []VideoTrack    `json:"video,omitempty"`
	Audio    []AudioTrack    `json:"audio,omitempty"`
	Subtitle []SubtitleTrack `json:"subtitle,omitempty"`
	Other    []TrackBase     `json:"other,omitempty"`
}

type MediaInfo struct {
	Container ContainerInfo `json:"container"`
	Tracks    TracksInfo    `json:"tracks"`
}
