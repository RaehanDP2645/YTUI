package ytdlp

type DownloadKind string

const (
	KindVideo DownloadKind = "video"
	KindMusic DownloadKind = "music"
)

type Mode string

const (
	ModeDefault Mode = "default"
	ModeCustom  Mode = "custom"
)

type Options struct {
	URL        string
	Kind       DownloadKind
	Mode       Mode
	Quality    string
	OutputDir  string
	FFmpegPath string
}

