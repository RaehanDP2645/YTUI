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
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"YTUI/internal/logger"
	"YTUI/internal/tools"
	"YTUI/internal/ytdlp"
)

const progressEventName = "download:progress"

const progressFlushInterval = 100 * time.Millisecond

var progressPattern = regexp.MustCompile(`\[download\]\s+(\d+(?:\.\d+)?)%.*?at\s+([^\s]+).*?ETA\s+([^\s]+)`)

// progressBatcher menggabungkan update progress yang datang sangat cepat dan
// mengirimnya ke frontend paling cepat sekali per progressFlushInterval.
// Update terakhir per item selalu diteruskan sehingga tidak ada progress
// terlewat, tapi jumlah event ke Wails/WebView2 tetap terbatas.
type progressBatcher struct {
	ctx  context.Context
	mu   sync.Mutex
	evt  *ProgressEvent
	sent *ProgressEvent
	done chan struct{}
}

func newProgressBatcher(ctx context.Context) *progressBatcher {
	b := &progressBatcher{
		ctx:  ctx,
		done: make(chan struct{}),
	}
	go b.loop()

	return b
}

func (b *progressBatcher) loop() {
	ticker := time.NewTicker(progressFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.flush()
		case <-b.done:
			return
		}
	}
}

func (b *progressBatcher) push(event ProgressEvent) {
	b.mu.Lock()
	b.evt = &event
	b.mu.Unlock()
}

func (b *progressBatcher) flush() {
	b.mu.Lock()
	if b.evt == nil || progressEventsEqual(*b.evt, b.sent) {
		b.mu.Unlock()
		return
	}
	event := *b.evt
	sent := event
	b.sent = &sent
	b.mu.Unlock()

	emitProgress(b.ctx, event)
}

func (b *progressBatcher) close() {
	close(b.done)
	b.flush()
}

func progressEventsEqual(a ProgressEvent, b *ProgressEvent) bool {
	return b != nil &&
		a.Status == b.Status &&
		a.Percent == b.Percent &&
		a.Speed == b.Speed &&
		a.ETA == b.ETA &&
		a.Message == b.Message
}

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

	quickJSPath := ""
	if p, err := findBinary(toolCandidates(toolsDir, "qjs")...); err == nil {
		quickJSPath = p
		logger.L.Runtime("quickjs path: %s", p)
	} else {
		logger.L.Runtime("qjs tidak ditemukan, pakai runtime bawaan yt-dlp: %v", err)
	}

	logger.L.Runtime("Download dimulai: %s -> %s", req.URL, outputDir)

	kind := mapDownloadKind(req.Type)

	args, err := ytdlp.BuildArgs(ytdlp.Options{
		URL:         req.URL,
		Kind:        kind,
		Mode:        ytdlp.ModeDefault,
		Quality:     req.Quality,
		OutputDir:   outputDir,
		FFmpegPath:  ffmpegPath,
		Aria2cPath:  aria2cPath,
		QuickJSPath: quickJSPath,
	})
	if err != nil {
		return DownloadResult{}, err
	}

	// Tentukan nama file output aktual terlebih dahulu (metadata saja) dan
	// periksa apakah file target sudah ada sebelum yt-dlp mulai men-download.
	targetPath, err := resolveOutputPath(ctx, ytDlpPath, args, kind)
	if err != nil {
		logger.L.Error("Gagal menentukan nama file output: %s - err: %v", req.URL, err)
		return DownloadResult{}, err
	}

	_, statErr := os.Stat(targetPath)
	logger.L.Runtime("[ConflictCheck] outputDir=%s filename=%s fullPath=%s exists=%v",
		filepath.Dir(targetPath), filepath.Base(targetPath), targetPath, statErr == nil)

	if statErr == nil {
		choice, err := handleExistingFile(ctx, targetPath)
		if err != nil {
			return DownloadResult{}, err
		}

		switch choice {
		case ExistingFileRename:
			renamed := uniqueExistingPath(targetPath)
			args = replaceOutputTemplate(args, renamed)
			logger.L.Runtime("File sudah ada, rename: %s -> %s", targetPath, renamed)
		case ExistingFileOverwrite:
			logger.L.Runtime("File sudah ada, overwrite: %s", targetPath)
		case ExistingFileAbort:
			emitProgress(ctx, ProgressEvent{
				URL:     req.URL,
				Status:  "canceled",
				Message: "File sudah ada, download dibatalkan",
			})
			logger.L.Runtime("File sudah ada, download dibatalkan: %s", targetPath)
			return DownloadResult{}, errors.New("download dibatalkan (file sudah ada)")
		}
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

	batcher := newProgressBatcher(ctx)
	defer batcher.close()

	var wg sync.WaitGroup
	var outputMu sync.Mutex
	var outputLines []string

	collect := func(line string) {
		outputMu.Lock()
		outputLines = append(outputLines, line)
		outputMu.Unlock()
	}

	wg.Add(2)
	go streamOutput(ctx, req.URL, stdout, collect, batcher.push, &wg)
	go streamOutput(ctx, req.URL, stderr, collect, batcher.push, &wg)

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

func streamOutput(ctx context.Context, url string, reader io.Reader, collect func(string), onProgress func(ProgressEvent), wg *sync.WaitGroup) {
	defer wg.Done()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		collect(line)

		if event, ok := parseProgressLine(url, line); ok {
			onProgress(event)
			continue
		}

		if strings.Contains(line, "[download] Destination:") {
			onProgress(ProgressEvent{
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
	if event, ok := parseStructuredProgress(url, line); ok {
		return event, true
	}

	return parseLegacyProgress(url, line)
}

const progressLinePrefix = "[PROG]|"

// parseStructuredProgress mem-parse baris progress terstruktur dari
// --progress-template. Formatnya: [PROG]|status|downloaded|total|totalEst|percent|speed|eta|speedStr|etaStr
func parseStructuredProgress(url string, line string) (ProgressEvent, bool) {
	if !strings.HasPrefix(line, progressLinePrefix) {
		return ProgressEvent{}, false
	}

	parts := strings.SplitN(line, "|", 10)
	if len(parts) < 10 {
		return ProgressEvent{}, false
	}

	downloaded, _ := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	total, _ := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
	totalEstimate, _ := strconv.ParseFloat(strings.TrimSpace(parts[4]), 64)

	if total <= 0 && totalEstimate > 0 {
		total = totalEstimate
	}

	var percent float64
	switch {
	case total > 0:
		percent = downloaded / total * 100
	default:
		if rawPercent, err := strconv.ParseFloat(strings.Trim(strings.TrimSpace(parts[5]), "%"), 64); err == nil {
			percent = rawPercent
		}
	}

	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	return ProgressEvent{
		URL:     url,
		Status:  "downloading",
		Percent: percent,
		Speed:   cleanProgressValue(parts[8]),
		ETA:     cleanProgressValue(parts[9]),
		Message: "Downloading",
		RawLine: line,
	}, true
}

func cleanProgressValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "N/A") || strings.EqualFold(value, "None") || strings.EqualFold(value, "Unknown") {
		return "-"
	}

	return value
}

func parseLegacyProgress(url string, line string) (ProgressEvent, bool) {
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

	// Dalam konteks batch, event item juga dicatat ke progressTracker dan
	// event overall dikirim setelahnya sehingga frontend bisa membedakan
	// progress item (event dengan URL) dari overall batch (status "overall").
	if tracker, ok := ctx.Value(progressTrackerKey).(*progressTracker); ok {
		tracker.record(event)
		tracker.emitOverall(ctx)
	}
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
