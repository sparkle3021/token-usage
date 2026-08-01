// Package debuglog 提供可开关的调试日志。
// [perf] 性能探针日志仅在环境变量 DEBUG_PERF=1 时输出，默认静默以降噪。
package debuglog

import (
	"log"
	"os"
)

var perfEnabled = os.Getenv("DEBUG_PERF") == "1"

// Perf 输出性能探针日志（DEBUG_PERF=1 时生效）。
func Perf(format string, args ...interface{}) {
	if perfEnabled {
		log.Printf("[perf] "+format, args...)
	}
}
