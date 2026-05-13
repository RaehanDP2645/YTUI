package downloader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"YTUI/internal/ytdlp"
)

func DownloadDefault(ctx context.Context, req DownloadRequest) (DownloadResult, error) {
	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		return DownloadResult{}, errors.New("URL tidak boleh kosong")
	}

	if req.Type == "" {
		req.Type = DownloadTypeVideo
	}

	outputDir := strings.TrimSpace(req.OutputDir)
	if outputDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return DownloadResult{}, err
		}
		outputDir = filepath.Join(home, "Downloads", "YTUI")
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return DownloadResult{}, err
	}

	ytDlpPath, err := findBinary("yt-dlp", "yt-dlp.exe", `C:\ytui-pro\yt-dlp.exe`)
	if err != nil {
		return DownloadResult{}, err
	}

	ffmpegPath, err := findBinary("ffmpeg", "ffmpeg.exe", `C:\ytui-pro\ffmpeg.exe`)
	if err != nil {
		return DownloadResult{}, err
	}

	args, err := ytdlp.BuildArgs(ytdlp.Options{
		URL:        req.URL,
		Kind:       mapDownloadKind(req.Type),
		Mode:       ytdlp.ModeDefault,
		OutputDir:  outputDir,
		FFmpegPath: ffmpegPath,
	})
	if err != nil {
		return DownloadResult{}, err
	}

	cmd := exec.CommandContext(ctx, ytDlpPath, args...)
	cmd.Dir = outputDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return DownloadResult{}, fmt.Errorf("yt-dlp gagal: %w\n%s", err, string(output))
	}

	return DownloadResult{
		Message:   "Download selesai",
		OutputDir: outputDir,
	}, nil
}

func mapDownloadKind(downloadType DownloadType) ytdlp.DownloadKind {
	switch downloadType {
	case DownloadTypeMusic:
		return ytdlp.KindMusic
	default:
		return ytdlp.KindVideo
	}
}

func findBinary(names ...string) (string, error) {
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}

		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
	}

	return "", fmt.Errorf("binary tidak ditemukan: %s", strings.Join(names, ", "))
}
