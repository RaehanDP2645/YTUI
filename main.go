package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"YTUI/internal/logger"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
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
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
