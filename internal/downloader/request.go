package downloader

type DownloadType string

const (
	DownloadTypeVideo DownloadType = "video"
	DownloadTypeMusic DownloadType = "music"
)

type DownloadRequest struct {
	URL       string       `json:"url"`
	Type      DownloadType `json:"type"`
	OutputDir string       `json:"outputDir"`
}

type DownloadResult struct {
	Message   string `json:"message"`
	OutputDir string `json:"outputDir"`
}
