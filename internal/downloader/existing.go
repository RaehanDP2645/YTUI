package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

var timestampRe = regexp.MustCompile(`[0-9]+(?::[0-9]+)+`)

// windowsSanitizeFilename adalah port dari sanitize_filename yt-dlp dengan
// restricted=false dan is_id=NO_DEFAULT (mode default saat argumen download
// memakai --windows-filenames tanpa --restrict-filenames). Dipakai agar hasil
// rekonstruksi path sama persis dengan nama file yang benar-benar dibuat oleh
// yt-dlp saat download.
func windowsSanitizeFilename(s string) string {
	if s == "" {
		return ""
	}

	s = timestampRe.ReplaceAllStringFunc(s, func(m string) string {
		return strings.ReplaceAll(m, ":", "_")
	})

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		b.WriteString(replaceInsane(r))
	}
	result := b.String()

	// Langkah post-process mengikuti sanitize_filename yt-dlp untuk
	// is_id=NO_DEFAULT: runtuhkan rangkaian \0X yang identik, buang karakter
	// pengganti (\0X, spasi, _, -) di awal/akhir hanya bila diapit \0.,
	// lalu hapus semua \0. Karena replaceInsane hanya menghasilkan "\0 ",
	// implementasinya cukup memakai operasi byte sederhana.
	result = collapseNulPairs(result)
	result = stripLeadingNul(result)
	result = stripTrailingNul(result)
	result = strings.ReplaceAll(result, "\x00", "")
	if result == "" {
		result = "_"
	}

	return result
}

// replaceInsane mengikuti replace_insane di sanitize_filename yt-dlp untuk
// restricted=false. Percabangan yang hanya aktif saat restricted=true atau
// is_id=False tidak diperlukan dan tidak diterjemahkan.
func replaceInsane(r rune) string {
	switch {
	case r == '\n':
		return "\x00 "
	case r == '/':
		return "\u29F8"
	case r == '\\':
		return "\u29F9"
	case strings.ContainsRune(`"*:<>?|`, r):
		return string(rune(r + 0xFEE0))
	case r < 32 || r == 127:
		return ""
	default:
		return string(r)
	}
}

// collapseNulPairs menyatukan rangkaian substitusi \0X yang berulang menjadi
// satu (padanan re.sub '(\0.)(?:(?=\1)..)+' -> '\1'). Karena hanya ada "\0 ",
// cukup runtuhkan run "\0 " berturut-turut.
func collapseNulPairs(s string) string {
	if !strings.Contains(s, "\x00") {
		return s
	}

	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x00 && i+1 < len(s) {
			b.WriteString(s[i : i+2])
			c := s[i+1]
			i += 2
			for i+1 < len(s) && s[i] == 0x00 && s[i+1] == c {
				i += 2
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func isSubstChar(b byte) bool {
	return b == ' ' || b == '_' || b == '-'
}

// stripLeadingNul adalah padanan '^\0.(?:\0.|[ _-])*'. Hanya berlaku bila
// string diawali \0.; karakter tanpa \0 di awal tidak diubah.
func stripLeadingNul(s string) string {
	if len(s) < 2 || s[0] != 0x00 {
		return s
	}

	i := 2
	for i < len(s) {
		if s[i] == 0x00 && i+1 < len(s) {
			i += 2
		} else if isSubstChar(s[i]) {
			i++
		} else {
			break
		}
	}
	if i > len(s) {
		i = len(s)
	}
	return s[i:]
}

// stripTrailingNul adalah padanan '(?:\0.|[ _-])*\0.$'. Hanya berlaku bila
// string diakhiri pasangan \0. (dua byte terakhir = \0X).
func stripTrailingNul(s string) string {
	n := len(s)
	if n < 2 || s[n-2] != 0x00 {
		return s
	}

	i := n - 2
	for i >= 2 {
		if s[i-2] == 0x00 {
			i -= 2
		} else if isSubstChar(s[i-1]) {
			i--
		} else {
			break
		}
	}
	return s[:i]
}

// titleEmptyInPath mendeteksi apakah bagian %(title)s pada hasil probe
// --print filename kosong. Template output selalu berbentuk
// "%(title)s - <suffix>.<ext>", sehingga title kosong menghasilkan basename
// yang diawali " - ".
func titleEmptyInPath(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, " - ")
}

// probeRealTitle menjalankan yt-dlp --dump-single-json untuk mendapatkan title
// final (dari info dict) yang tidak tersedia pada ekspansi awal --print.
func probeRealTitle(ctx context.Context, ytDlpPath string, downloadArgs []string) (string, error) {
	url := downloadArgs[len(downloadArgs)-1]
	probeArgs := make([]string, 0, len(downloadArgs)+1)
	probeArgs = append(probeArgs, downloadArgs[:len(downloadArgs)-1]...)
	probeArgs = append(probeArgs, "--dump-single-json", url)

	cmd := exec.CommandContext(ctx, ytDlpPath, probeArgs...)
	hideWindow(cmd)

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("dump-single-json gagal: %w", err)
	}

	var info struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		return "", fmt.Errorf("parse info json gagal: %w", err)
	}

	return info.Title, nil
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

	// Beberapa video (umumnya judul non-ASCII) baru mengisi title di info dict
	// final, sehingga ekspansi %(title)s pada --print filename menghasilkan
	// string kosong dan basename diawali " - ". Fallback: ambil title real dari
	// --dump-single-json lalu rekonstruksi path agar sama dengan file yang akan
	// dibuat download sungguhan.
	if titleEmptyInPath(path) {
		logger.L.Runtime("[ConflictCheck] Title kosong dalam probe, mengambil title dari info dict: %s", path)
		if realTitle, err := probeRealTitle(ctx, ytDlpPath, downloadArgs); err == nil && realTitle != "" {
			dir := filepath.Dir(path)
			base := filepath.Base(path)
			path = filepath.Join(dir, windowsSanitizeFilename(realTitle)+base)
			logger.L.Runtime("[ConflictCheck] Path dikoreksi: %s", path)
		} else {
			logger.L.Runtime("[ConflictCheck] Gagal mengambil title real: %v", err)
		}
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
