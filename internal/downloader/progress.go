package downloader

type ProgressEvent struct {
	URL      string  `json:"url"`
	Status   string  `json:"status"`
	Percent  float64 `json:"percent"`
	Speed    string  `json:"speed"`
	ETA      string  `json:"eta"`
	Message  string  `json:"message"`
	RawLine  string  `json:"rawLine"`
}
