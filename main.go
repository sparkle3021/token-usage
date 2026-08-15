package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	setupLogging()

	// 单实例：已有实例时通知其显示主窗口，然后退出本进程
	release, err := acquireSingleInstance()
	if err != nil {
		log.Printf("[main] single-instance check: %v", err)
		activateExistingInstance()
		return
	}
	defer release()

	app := NewApp()
	startTray(app)         // 系统托盘（独立 goroutine）
	go watchShowEvent(app) // 第二实例激活信号监听

	// Create application with options
	err = wails.Run(&options.App{
		Title:     "TokenUsage",
		Width:     900,
		Height:    620,
		MinWidth:  900,
		MinHeight: 620,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 243, G: 243, B: 243, A: 1},
		Windows: &windows.Options{
			Theme: windows.SystemDefault,
		},
		OnStartup:     app.startup,
		OnShutdown:    app.shutdown,
		OnBeforeClose: app.onBeforeClose, // 点 × 隐藏到托盘；托盘退出时放行
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
