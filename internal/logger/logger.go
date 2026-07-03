// Package logger 提供统一日志封装，基于 slog，支持分级控制。
// 默认 Info 级别，可通过 SetLevel 动态调整。
package logger

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"
)

var levelVar = new(slog.LevelVar)

// Init 初始化日志系统，设置日志级别。
// 默认 Info 级别，输出到标准输出，包含短文件路径和行号。
func Init(level slog.Level) {
	levelVar.Set(level)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:     levelVar,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				if source, ok := a.Value.Any().(*slog.Source); ok {
					// Only show file:line relative to project root
					short := source.File
					for i := 0; i < 4; i++ {
						if idx := strings.IndexByte(short, '/'); idx >= 0 {
							short = short[idx+1:]
						}
					}
					a.Value = slog.StringValue(short)
				}
			}
			return a
		},
	})))
}

func init() {
	Init(slog.LevelInfo)
}

func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])
	r := slog.NewRecord(time.Now(), slog.LevelError, msg, pcs[0])
	r.Add(args...)
	_ = slog.Default().Handler().Handle(context.Background(), r)
}

func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

func SetLevel(level slog.Level) {
	levelVar.Set(level)
}
