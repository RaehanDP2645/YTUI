package downloader

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"YTUI/internal/ytdlp"
)

const progressEventName = "download:progress"

var progressPattern = regexp.MustCompile(`\[download\]\s+(\d+(?:\.\d+)?)%.*?at\s+([^\s]+).*?ETA\s+([^\s]+)`)

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
		Quality:    req.Quality,
		OutputDir:  outputDir,
		FFmpegPath: ffmpegPath,
	})
	if err != nil {
		return DownloadResult{}, err
	}

	emitProgress(ctx, ProgressEvent{
		URL:     req.URL,
		Status:  "downloading",
		Message: "Memulai download",
	})

	cmd := exec.CommandContext(ctx, ytDlpPath, args...)
	cmd.Dir = outputDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return DownloadResult{}, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return DownloadResult{}, err
	}

	if err := cmd.Start(); err != nil {
		return DownloadResult{}, err
	}

	var wg sync.WaitGroup
	var outputMu sync.Mutex
	var outputLines []string

	collect := func(line string) {
		outputMu.Lock()
		outputLines = append(outputLines, line)
		outputMu.Unlock()
	}

	wg.Add(2)
	go streamOutput(ctx, req.URL, stdout, collect, &wg)
	go streamOutput(ctx, req.URL, stderr, collect, &wg)

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		outputMu.Lock()
		outputText := strings.Join(outputLines, "\n")
		outputMu.Unlock()

		emitProgress(ctx, ProgressEvent{
			URL:     req.URL,
			Status:  "failed",
			Message: "Download gagal",
		})

		return DownloadResult{}, fmt.Errorf("yt-dlp gagal: %w\n%s", err, outputText)
	}

	emitProgress(ctx, ProgressEvent{
		URL:     req.URL,
		Status:  "completed",
		Percent: 100,
		Message: "Download selesai",
	})

	return DownloadResult{
		Message:   "Download selesai",
		OutputDir: outputDir,
	}, nil
}

func streamOutput(ctx context.Context, url string, reader io.Reader, collect func(string), wg *sync.WaitGroup) {
	defer wg.Done()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		collect(line)

		if event, ok := parseProgressLine(url, line); ok {
			emitProgress(ctx, event)
			continue
		}

		if strings.Contains(line, "[download] Destination:") {
			emitProgress(ctx, ProgressEvent{
				URL:     url,
				Status:  "downloading",
				Message: "Menyiapkan file output",
				RawLine: line,
			})
		}
	}
}

func parseProgressLine(url string, line string) (ProgressEvent, bool) {
	matches := progressPattern.FindStringSubmatch(line)
	if len(matches) != 4 {
		return ProgressEvent{}, false
	}

	percent, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return ProgressEvent{}, false
	}

	return ProgressEvent{
		URL:     url,
		Status:  "downloading",
		Percent: percent,
		Speed:   matches[2],
		ETA:     matches[3],
		Message: "Downloading",
		RawLine: line,
	}, true
}

func emitProgress(ctx context.Context, event ProgressEvent) {
	runtime.EventsEmit(ctx, progressEventName, event)
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
