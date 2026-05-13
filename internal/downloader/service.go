package downloader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	args := buildDefaultArgs(req.Type, outputDir, ffmpegPath, req.URL)

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

func buildDefaultArgs(downloadType DownloadType, outputDir string, ffmpegPath string, url string) []string {
	outputTemplate := filepath.Join(outputDir, "%(title)s.%(ext)s")

	baseArgs := []string{
		"--newline",
		"--no-playlist",
		"--ffmpeg-location", ffmpegPath,
		"-o", outputTemplate,
	}

	switch downloadType {
	case DownloadTypeMusic:
		return append(baseArgs,
			"-f", "bestaudio",
			"-x",
			"--audio-format", "mp3",
			"--embed-metadata",
			"--embed-thumbnail",
			url,
		)
	default:
		return append(baseArgs,
			"-f", "bestvideo+bestaudio/best",
			"--merge-output-format", "mp4",
			url,
		)
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
