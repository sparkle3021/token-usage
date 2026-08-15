//go:build darwin

package main

import (
	_ "embed"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"token-dashboard/internal/config"

	"github.com/getlantern/systray"
)

// 托盘图标（macOS：PNG 模板图标，随菜单栏深浅色自动适配）
//
//go:embed build/appicon.png
var trayIconBytes []byte

// setupTrayIcon 设置托盘图标。平台差异封装于此，tray.go 只调用不感知平台。
func setupTrayIcon() {
	if len(trayIconBytes) > 0 {
		systray.SetTemplateIcon(trayIconBytes, trayIconBytes)
	}
}

// macOS 单实例：flock 文件锁 + unix socket 窗口激活。
// Windows 版用命名内核对象（见 platform_windows.go），此处语义等价：
//   - 锁：非阻塞 flock，拿不到即已有实例
//   - 激活：第一实例常驻监听 unix socket，第二实例写入 "show" 唤起窗口

const showSocketName = "app-show.sock"

func appLockPath() string { return filepath.Join(config.Load().DataDir, "app.lock") }
func appSockPath() string { return filepath.Join(config.Load().DataDir, showSocketName) }

// acquireSingleInstance 获取单实例文件锁。已有实例运行时返回错误（第二实例路径）。
func acquireSingleInstance() (func(), error) {
	f, err := os.OpenFile(appLockPath(), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("创建单实例锁文件失败: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("TokenUsage 已在运行")
	}
	// 锁文件保留不删除：删除会产生竞态（旧 inode 上的锁形同虚设，第三实例可新建文件绕过锁）。
	// 锁状态挂在内核 fd 上，进程退出自动释放。
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

// activateExistingInstance 通过 unix socket 通知已运行实例显示主窗口（第二实例路径）。
// 失败（如第一实例恰好退出）仅记日志，静默返回。
func activateExistingInstance() {
	conn, err := net.Dial("unix", appSockPath())
	if err != nil {
		log.Printf("[single] dial show socket failed: %v", err)
		return
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("show")); err != nil {
		log.Printf("[single] write show socket failed: %v", err)
	}
}

// watchShowEvent 常驻监听 unix socket，收到 "show" 后唤起主窗口。
// socket 故障只影响"激活"增强功能，单实例锁与托盘不受影响。
func watchShowEvent(app *App) {
	// 清理陈旧 socket（第一实例强杀后残留）；单实例锁保证当前进程是唯一监听者
	_ = os.Remove(appSockPath())
	ln, err := net.Listen("unix", appSockPath())
	if err != nil {
		log.Printf("[single] listen show socket failed: %v", err)
		return
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("[single] accept show socket failed: %v", err)
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			buf := make([]byte, 8)
			n, _ := c.Read(buf)
			if n > 0 && string(buf[:n]) == "show" {
				log.Println("[single] show-event received, activating window")
				app.showWindow()
			}
		}(conn)
	}
}
