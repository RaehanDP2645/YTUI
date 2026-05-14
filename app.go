package main

import (
	"YTUI/internal/downloader"
	"context"
	"fmt"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx context.Context
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
	return downloader.DownloadDefault(a.ctx, req)
}

func (a *App) SelectBatchFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Pilih file batch .txt",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Text Files (*.txt)",
				Pattern:     "*.txt",
			},
		},
	})
}

func (a *App) DownloadBatch(req downloader.BatchDownloadRequest) (downloader.BatchDownloadResult, error) {
	return downloader.DownloadBatch(a.ctx, req)
}
