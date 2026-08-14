package main

import (
	"embed"
	"fmt"
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
	logPath := filepath.Join(logDir, "app.log")
	rotateLog(logPath, 5<<20, 3) // 5MB 大小轮转，保留 3 份
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("[main] setupLogging open file error: %v", err)
		return
	}
	// 注意：不能用 io.MultiWriter(os.Stderr, f)——GUI 应用（无控制台）的
	// os.Stderr 写失败时 MultiWriter 会短路，文件日志全部丢失（双击启动
	// 时 app.log 一直为空就是这个原因）。
	log.SetOutput(&logTee{file: f})
	log.Printf("[main] logging initialized dir=%s", logDir)
}

// logTee 双写 stderr 与文件；stderr 失败（GUI 应用无有效 stderr）不影响文件日志。
type logTee struct{ file *os.File }

func (w *logTee) Write(p []byte) (int, error) {
	if _, err := os.Stderr.Write(p); err != nil {
		// ignore: GUI 应用无控制台
	}
	return w.file.Write(p)
}

// rotateLog 按大小轮转日志：超过 maxSize 字节时归档为 .1/.2，保留 keep 份。
func rotateLog(path string, maxSize int64, keep int) {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() < maxSize {
		return
	}
	for i := keep - 1; i >= 1; i-- {
		os.Rename(fmt.Sprintf("%s.%d", path, i), fmt.Sprintf("%s.%d", path, i+1))
	}
	os.Rename(path, path+".1")
}

func main() {
	setupLogging()

	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
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
