package ytdlp

import (
	"errors"
	"path/filepath"
	"strings"

	// "golang.org/x/text/cases"
)

func BuildArgs(opts Options) ([]string, error) {
	opts.URL = strings.TrimSpace(opts.URL)

	if opts.URL == "" {
		return nil, errors.New("URL tidak boleh kosong")
	}

	if opts.OutputDir == "" {
		return nil, errors.New("output folder tidak boleh kosong")
	}

	if opts.FFmpegPath == "" {
		return nil, errors.New("path ffmpeg tidak boleh kosong")
	}

	if opts.Kind == "" {
		opts.Kind = KindVideo
	}

	if opts.Mode == "" {
		opts.Mode = ModeDefault
	}

	if opts.Mode != ModeDefault {
		return nil, errors.New("custom mode belum tersedia di tahap ini")
	}

	return buildDefaultArgs(opts), nil
}

func buildDefaultArgs(opts Options) []string {
	outputTemplate := buildOutputTemplate(opts)

	baseArgs := []string{
		"--newline",
		"--no-playlist",
		"--windows-filenames",
		"--ffmpeg-location", opts.FFmpegPath,
		"-o", outputTemplate,
	}

	switch opts.Kind {
	case KindMusic:
		args := append(baseArgs,
			"-f", "bestaudio",
			"-x",
			"--audio-format", "mp3",
			"--embed-metadata",
			"--embed-thumbnail",
		)

		args = append(args, buildAudioQualityArgs(opts.Quality)...)
		args = append(args, opts.URL)

		return args

	default:
		return append(baseArgs,
			"-f", buildVideoFormat(opts.Quality),
			"--merge-output-format", "mp4",
			opts.URL,
		)
	}
}

func buildOutputTemplate(opts Options) string {
	suffix := buildQualitySuffix(opts.Kind, opts.Quality)

	filenameTemplate := "%(title)s"
	if suffix != "" {
		filenameTemplate += " - " + suffix
	}

	filenameTemplate += ".%(ext)s"

	return filepath.Join(opts.OutputDir, filenameTemplate)
}

func buildQualitySuffix(kind DownloadKind, quality string) string {
	quality = strings.TrimSpace(strings.ToLower(quality))

	if kind == KindMusic {
		switch quality {
			case "320", "320k":
				return "320K"
			case "256", "256k":
				return "256K"
			case "192", "192k":
				return "192K"
			case "128", "128k":
				return "128K"
			default:
				return "best"
		}
	}

	switch quality {
		case "1080", "1080p":
			return "1080p"
		case "720", "720p":
			return "720p"
		case "480", "480p":
			return "480p"
		case "360", "360p":
			return "360p"
		default:
			return "best"
	}
}

func buildVideoFormat(quality string) string {
	switch strings.TrimSpace(strings.ToLower(quality)) {
	case "1080", "1080p":
		return "bestvideo[height<=1080]+bestaudio/best[height<=1080]"
	case "720", "720p":
		return "bestvideo[height<=720]+bestaudio/best[height<=720]"
	case "480", "480p":
		return "bestvideo[height<=480]+bestaudio/best[height<=480]"
	case "360", "360p":
		return "bestvideo[height<=360]+bestaudio/best[height<=360]"
	default:
		return "bestvideo+bestaudio/best"
	}
}

func buildAudioQualityArgs(quality string) []string {
	switch quality {
	case "320":
		return []string{"--audio-quality", "320K"}
	case "256":
		return []string{"--audio-quality", "256K"}
	case "192":
		return []string{"--audio-quality", "192K"}
	case "128":
		return []string{"--audio-quality", "128K"}
	default:
		return nil
	}
}
