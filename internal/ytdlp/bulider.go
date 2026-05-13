package ytdlp

import (
	"errors"
	"path/filepath"
	"strings"
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
	outputTemplate := filepath.Join(opts.OutputDir, "%(title)s.%(ext)s")

	baseArgs := []string{
		"--newline",
		"--no-playlist",
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
