package main

import (
	"YTUI/internal/downloader"
	"YTUI/internal/logger"
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
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
