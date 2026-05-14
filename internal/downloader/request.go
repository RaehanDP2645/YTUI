package downloader

type DownloadType string

const (
	DownloadTypeVideo DownloadType = "video"
	DownloadTypeMusic DownloadType = "music"
)

type DownloadRequest struct {
	URL       string       `json:"url"`
	Type      DownloadType `json:"type"`
	Quality   string       `json:"quality"`
	OutputDir string       `json:"outputDir"`
}

type DownloadResult struct {
	Message   string `json:"message"`
	OutputDir string `json:"outputDir"`
}

type BatchDownloadRequest struct {
	FilePath   string       `json:"filePath"`
	Type       DownloadType `json:"type"`
	Quality    string       `json:"quality"`
	OutputDir  string       `json:"outputDir"`
	Parallel   int          `json:"parallel"`
	SkipErrors bool         `json:"skipErrors"`
}

type BatchDownloadResult struct {
	Message   string `json:"message"`
	Total     int    `json:"total"`
	Completed int    `json:"completed"`
	Failed    int    `json:"failed"`
	OutputDir string `json:"outputDir"`
}
