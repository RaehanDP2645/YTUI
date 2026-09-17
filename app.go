package main

import (
	"YTUI/internal/downloader"
	"YTUI/internal/logger"
	"context"
	"encoding/base64"
	"fmt"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const fileExistsAskEvent = "file-exists:ask"

// App struct
type App struct {
	ctx    context.Context
	cancel context.CancelFunc

	// fileExistsMu melindungi state dialog "file sudah ada".
	fileExistsMu     sync.Mutex
	fileExistsWaits  map[string]chan downloader.ExistingFileChoice
	fileExistsPolicy downloader.ExistingFileChoice
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		fileExistsWaits:  make(map[string]chan downloader.ExistingFileChoice),
		fileExistsPolicy: downloader.ExistingFileAsk,
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	downloader.SetExistingFileHandler(a.resolveExistingFile)
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) DownloadDefault(req downloader.DownloadRequest) (downloader.DownloadResult, error) {
	downloadCtx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel

	defer func() {
		a.cancel = nil
	}()

	return downloader.DownloadDefault(downloadCtx, req)
}

func (a *App) CancelDownload() string {
	logger.L.Runtime("User membatalkan download")

	if a.cancel == nil {
		return "Tidak ada download aktif"
	}

	a.cancel()
	return "Download dibatalkan"
}

func (a *App) SelectBatchFile() (string, error) {
	filepath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Pilih file batch .txt",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Text Files (*.txt)",
				Pattern:     "*.txt",
			},
		},
	})

	if filepath != "" {
		logger.L.Runtime("Batch file dipilih: %s", filepath)
	}

	return filepath, err
}

func (a *App) DownloadBatch(req downloader.BatchDownloadRequest) (downloader.BatchDownloadResult, error) {
	logger.L.Runtime("Batch download dipanggil dari UI: %s", req.FilePath)

	return downloader.DownloadBatch(a.ctx, req)
}

// resolveExistingFile dipanggil oleh downloader saat file output sudah ada.
// Memakai kebijakan yang diingat ("ask" default), atau menampilkan dialog
// ke frontend dan menunggu jawaban user.
func (a *App) resolveExistingFile(ctx context.Context, path string) (downloader.ExistingFileChoice, error) {
	a.fileExistsMu.Lock()
	policy := a.fileExistsPolicy
	a.fileExistsMu.Unlock()

	if policy != downloader.ExistingFileAsk {
		return policy, nil
	}

	id := "fe-" + base64.RawURLEncoding.EncodeToString([]byte(path))
	ch := make(chan downloader.ExistingFileChoice, 1)

	a.fileExistsMu.Lock()
	a.fileExistsWaits[id] = ch
	a.fileExistsMu.Unlock()

	defer func() {
		a.fileExistsMu.Lock()
		delete(a.fileExistsWaits, id)
		a.fileExistsMu.Unlock()
	}()

	runtime.EventsEmit(ctx, fileExistsAskEvent, map[string]string{
		"id":   id,
		"path": path,
	})

	select {
	case choice := <-ch:
		return choice, nil
	case <-ctx.Done():
		return downloader.ExistingFileAbort, ctx.Err()
	}
}

// ResolveFileExists menerima jawaban user dari dialog "file sudah ada".
// remember=true menyimpan pilihan untuk dipakai pada download berikutnya.
func (a *App) ResolveFileExists(id string, choice string, remember bool) {
	c, ok := downloader.ParseExistingFileChoice(choice)
	if !ok {
		return
	}

	a.fileExistsMu.Lock()

	if ch, ok := a.fileExistsWaits[id]; ok {
		select {
		case ch <- c:
		default:
		}
	}

	if remember {
		a.fileExistsPolicy = c
		logger.L.Runtime("Pilihan file-sudah-ada diingat: %s", c)
	}

	a.fileExistsMu.Unlock()
}

// GetFileExistsPolicy mengembalikan kebijakan "file sudah ada" saat ini.
func (a *App) GetFileExistsPolicy() string {
	a.fileExistsMu.Lock()
	defer a.fileExistsMu.Unlock()

	return a.fileExistsPolicy.String()
}

// SetFileExistsPolicy mengubah kebijakan "file sudah ada" ("ask" agar dialog
// muncul lagi).
func (a *App) SetFileExistsPolicy(policy string) {
	c, ok := downloader.ParseExistingFileChoice(policy)
	if !ok {
		return
	}

	a.fileExistsMu.Lock()
	a.fileExistsPolicy = c
	a.fileExistsMu.Unlock()
}
