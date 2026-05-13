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
		return append(baseArgs,
			"-f", "bestaudio",
			"-x",
			"--audio-format", "mp3",
			"--embed-metadata",
			"--embed-thumbnail",
			opts.URL,
		)
	default:
		return append(baseArgs,
			"-f", "bestvideo+bestaudio/best",
			"--merge-output-format", "mp4",
			opts.URL,
		)
	}
}
