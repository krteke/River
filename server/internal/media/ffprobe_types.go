package media

type ffprobeOutput struct {
	Format  ffprobeFormat   `json:"format"`
	Streams []ffprobeStream `json:"streams"`
}

type ffprobeFormat struct {
	FormatName string            `json:"format_name"`
	Duration   string            `json:"duration"`
	BitRate    string            `json:"bit_rate"`
	Size       string            `json:"size"`
	StartTime  string            `json:"start_time"`
	Tags       map[string]string `json:"tags"`
}

type ffprobeStream struct {
	Index          int               `json:"index"`
	CodecName      string            `json:"codec_name"`
	CodecType      string            `json:"codec_type"`
	Profile        string            `json:"profile"`
	Width          int               `json:"width"`
	Height         int               `json:"height"`
	PixFmt         string            `json:"pix_fmt"`
	AvgFrameRate   string            `json:"avg_frame_rate"`
	RFrameRate     string            `json:"r_frame_rate"`
	TimeBase       string            `json:"time_base"`
	Duration       string            `json:"duration"`
	BitRate        string            `json:"bit_rate"`
	ColorRange     string            `json:"color_range"`
	ColorSpace     string            `json:"color_space"`
	ColorTransfer  string            `json:"color_transfer"`
	ColorPrimaries string            `json:"color_primaries"`
	SampleRate     string            `json:"sample_rate"`
	Channels       int               `json:"channels"`
	ChannelLayout  string            `json:"channel_layout"`
	Disposition    map[string]int    `json:"disposition"`
	Tags           map[string]string `json:"tags"`
}
