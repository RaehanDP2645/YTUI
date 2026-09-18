package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"YTUI/internal/logger"
	"YTUI/internal/singleinstance"
	"YTUI/internal/tools"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed all:bin
var binScripts embed.FS

func main() {
	if singleinstance.Acquire() {
		singleinstance.ShowDialog()
		return
	}
	defer singleinstance.Release()

	// Create an instance of the app structure
	app := NewApp()

	var err error
	logger.L, err = logger.New("logs")
	if err != nil {
		println("Gagal init logger:", err.Error())
	} else {
		defer logger.L.Close()
		logger.L.Runtime("App YTUI dimulai")
	}

	if toolsDir, err := tools.Extract(binScripts); err != nil {
		if logger.L != nil {
			logger.L.Error("Gagal ekstrak tools: %v", err)
		}
	} else if toolsDir != "" {
		logger.L.Runtime("Tools siap di: %s", toolsDir)
	}
	// Create application with options
	err = wails.Run(&options.App{
		Title:  "YTUI - YouTube Downloader",
		Width:  980,
		Height: 620,

		MinWidth:  860,
		MinHeight: 540,

		DisableResize: false,
		Fullscreen:    false,
		Frameless:     false,

		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 244, G: 246, B: 248, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
