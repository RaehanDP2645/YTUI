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

	"YTUI/internal/logger"
	"YTUI/internal/tools"
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

	toolsDir, _ := tools.Dir()

	ytDlpPath, err := findBinary(toolCandidates(toolsDir, "yt-dlp")...)
	if err != nil {
		return DownloadResult{}, err
	}
	logger.L.Runtime("yt-dlp path: %s", ytDlpPath)

	ffmpegPath, err := findBinary(toolCandidates(toolsDir, "ffmpeg")...)
	if err != nil {
		return DownloadResult{}, err
	}
	logger.L.Runtime("ffmpeg path: %s", ffmpegPath)

	aria2cPath := ""
	if p, err := findBinary(toolCandidates(toolsDir, "aria2c")...); err == nil {
		aria2cPath = p
		logger.L.Runtime("aria2c path: %s", p)
	} else {
		logger.L.Runtime("aria2c tidak ditemukan, pakai downloader bawaan: %v", err)
	}

	denoPath := ""
	if p, err := findBinary(toolCandidates(toolsDir, "deno")...); err == nil {
		denoPath = p
		logger.L.Runtime("deno path: %s", p)
	} else {
		logger.L.Runtime("deno tidak ditemukan, pakai node/yang ada: %v", err)
	}

	logger.L.Runtime("Download dimulai: %s -> %s", req.URL, outputDir)

	args, err := ytdlp.BuildArgs(ytdlp.Options{
		URL:        req.URL,
		Kind:       mapDownloadKind(req.Type),
		Mode:       ytdlp.ModeDefault,
		Quality:    req.Quality,
		OutputDir:  outputDir,
		FFmpegPath: ffmpegPath,
		Aria2cPath: aria2cPath,
		DenoPath:   denoPath,
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
	hideWindow(cmd)

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
		if errors.Is(ctx.Err(), context.Canceled) {
			emitProgress(ctx, ProgressEvent{
				URL:     req.URL,
				Status:  "canceled",
				Message: "Download dibatalkan",
			})
			logger.L.Error("Download dibatalkan: %s", req.URL)

			return DownloadResult{}, errors.New("download dibatalkan")
		}

		emitProgress(ctx, ProgressEvent{
			URL:     req.URL,
			Status:  "failed",
			Message: "Download gagal",
		})
		logger.L.Error("Download gagal: %s - err: %v", req.URL, err)

		outputMu.Lock()
		if len(outputLines) > 0 {
			logger.L.Error("yt-dlp output:")
			for _, line := range outputLines {
				logger.L.Error("  | %s", line)
			}
		}
		outputMu.Unlock()

		return DownloadResult{}, errors.New("download gagal")
	}

	emitProgress(ctx, ProgressEvent{
		URL:     req.URL,
		Status:  "completed",
		Percent: 100,
		Message: "Download selesai",
	})
	logger.L.Runtime("Download selesai: %s", req.URL)

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

	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return
		}

		collect("Gagal membaca output: " + err.Error())
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

func toolCandidates(toolsDir, exe string) []string {
	var candidates []string

	candidates = append(candidates,
		filepath.Join("bin", exe),
		filepath.Join("bin", exe+".exe"),
	)

	if toolsDir != "" {
		candidates = append(candidates,
			filepath.Join(toolsDir, exe),
			filepath.Join(toolsDir, exe+".exe"),
		)
	}

	candidates = append(candidates, exe, exe+".exe")

	return candidates
}

func findBinary(names ...string) (string, error) {
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			if abs, err := filepath.Abs(path); err == nil {
				return abs, nil
			}
			return path, nil
		}

		if _, err := os.Stat(name); err == nil {
			if abs, err := filepath.Abs(name); err == nil {
				return abs, nil
			}
			return name, nil
		}
	}

	return "", fmt.Errorf("binary tidak ditemukan: %s", strings.Join(names, ", "))
}
