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
	"strings"
	"sync"

	"YTUI/internal/logger"
	"YTUI/internal/ytdlp"
)

// ExistingFileChoice menentukan aksi bila file output sudah ada.
type ExistingFileChoice uint8

const (
	// ExistingFileAsk mintalah user lewat dialog sebelum melanjutkan.
	ExistingFileAsk ExistingFileChoice = iota
	// ExistingFileRename gunakan nama file berikutnya yang tersedia.
	ExistingFileRename
	// ExistingFileOverwrite lanjutkan download dan biarkan file lama tertimpa.
	ExistingFileOverwrite
	// ExistingFileAbort batalkan download tersebut.
	ExistingFileAbort
)

func (c ExistingFileChoice) String() string {
	switch c {
	case ExistingFileRename:
		return "rename"
	case ExistingFileOverwrite:
		return "overwrite"
	case ExistingFileAbort:
		return "abort"
	default:
		return "ask"
	}
}

// ParseExistingFileChoice mengubah nama string menjadi ExistingFileChoice.
func ParseExistingFileChoice(s string) (ExistingFileChoice, bool) {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "ask":
		return ExistingFileAsk, true
	case "rename":
		return ExistingFileRename, true
	case "overwrite":
		return ExistingFileOverwrite, true
	case "abort":
		return ExistingFileAbort, true
	default:
		return ExistingFileAsk, false
	}
}

// ExistingFileHandler dipanggil saat file output sudah ada. Implementasi
// disediakan oleh lapisan App (mengatur dialog Wails / keputusan yang diingat).
type ExistingFileHandler func(ctx context.Context, path string) (ExistingFileChoice, error)

var (
	existingFileHandlerMu sync.RWMutex
	existingFileHandler   ExistingFileHandler
)

// SetExistingFileHandler memasang handler keputusan "file sudah ada".
// Cukup dipanggil sekali saat aplikasi dimulai.
func SetExistingFileHandler(handler ExistingFileHandler) {
	existingFileHandlerMu.Lock()
	defer existingFileHandlerMu.Unlock()
	existingFileHandler = handler
}

func handleExistingFile(ctx context.Context, path string) (ExistingFileChoice, error) {
	existingFileHandlerMu.RLock()
	handler := existingFileHandler
	existingFileHandlerMu.RUnlock()

	if handler == nil {
		// Default aman: jangan menimpa file secara diam-diam.
		return ExistingFileRename, nil
	}

	return handler(ctx, path)
}

// resolveOutputPath menjalankan yt-dlp (metadata saja, tanpa download) untuk
// mengetahui nama file output aktual sebelum proses download dimulai. Argumen
// probe sama persis dengan argumen download agar nama yang dihasilkan pasti
// sama, hanya ditambah "--print filename".
func resolveOutputPath(ctx context.Context, ytDlpPath string, downloadArgs []string, kind ytdlp.DownloadKind) (string, error) {
	if len(downloadArgs) == 0 {
		return "", errors.New("argumen yt-dlp kosong")
	}

	url := downloadArgs[len(downloadArgs)-1]
	probeArgs := make([]string, 0, len(downloadArgs)+2)
	probeArgs = append(probeArgs, downloadArgs[:len(downloadArgs)-1]...)
	probeArgs = append(probeArgs, "--print", "filename", url)

	cmd := exec.CommandContext(ctx, ytDlpPath, probeArgs...)
	hideWindow(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	var (
		mu       sync.Mutex
		outLines []string
		errLines []string
		wg       sync.WaitGroup
	)

	wg.Add(2)
	go scanOutputLines(stdout, &mu, &outLines, &wg)
	go scanOutputLines(stderr, &mu, &errLines, &wg)
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		mu.Lock()
		for _, line := range errLines {
			logger.L.Error("  | %s", line)
		}
		mu.Unlock()

		return "", fmt.Errorf("yt-dlp gagal menentukan nama file output: %w", err)
	}

	var path string
	mu.Lock()
	for i := len(outLines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(outLines[i])
		if line != "" && !strings.EqualFold(line, "NA") {
			path = line
			break
		}
	}
	mu.Unlock()

	if path == "" {
		return "", errors.New("yt-dlp tidak mengembalikan nama file output")
	}

	// Untuk mode music, yt-dlp melakukan ekstraksi audio (-x --audio-format mp3)
	// sehingga --print filename menunjukkan file sumber (mis. .webm/.m4a),
	// padahal file final yang dibuat adalah .mp3.
	if kind == ytdlp.KindMusic {
		path = strings.TrimSuffix(path, filepath.Ext(path)) + ".mp3"
	}

	logger.L.Runtime("Output target: %s", path)
	return path, nil
}

func scanOutputLines(reader io.Reader, mu *sync.Mutex, lines *[]string, wg *sync.WaitGroup) {
	defer wg.Done()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		mu.Lock()
		*lines = append(*lines, scanner.Text())
		mu.Unlock()
	}
}

// renameMu mencegah dua download memilih nama " (N)" yang sama bersamaan
// dalam batch/parallel.
var renameMu sync.Mutex

// uniqueExistingPath mencari nama file berikutnya yang benar-benar belum ada,
// contoh: video.mp4 -> video (1).mp4 -> video (2).mp4 dst.
func uniqueExistingPath(path string) string {
	renameMu.Lock()
	defer renameMu.Unlock()

	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}

	for i := 1; i < 1000; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}

	return path
}

// replaceOutputTemplate mengganti nilai argumen "-o" pada yt-dlp.
func replaceOutputTemplate(args []string, newTemplate string) []string {
	out := make([]string, len(args))
	copy(out, args)

	for i := 0; i < len(out); i++ {
		if out[i] == "-o" && i+1 < len(out) {
			out[i+1] = newTemplate
			break
		}
	}

	return out
}
