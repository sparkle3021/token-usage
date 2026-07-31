package main

import (
	"embed"
	"io"
	"log"
	"os"
	"path/filepath"

	"token-dashboard/internal/config"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func setupLogging() {
	cfg := config.Load()
	logDir := filepath.Join(cfg.DataDir, "logs")
	os.MkdirAll(logDir, 0755)
	f, err := os.OpenFile(filepath.Join(logDir, "app.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("[main] setupLogging open file error: %v", err)
		return
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.Printf("[main] logging initialized dir=%s", logDir)
}

func main() {
	setupLogging()

	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "TokenUsage",
		Width:     900,
		Height:    600,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 243, G: 243, B: 243, A: 1},
		Windows: &windows.Options{
			Theme: windows.SystemDefault,
		},
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
