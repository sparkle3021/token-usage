//go:build windows

package main

import (
	"fmt"
	"log"

	"golang.org/x/sys/windows"
)

const (
	instanceMutexName = `Local\TokenUsage.SingleInstance`
	showEventName     = `Local\TokenUsage.ShowMainWindow`
)

// acquireSingleInstance 获取单实例互斥锁。已有实例运行时返回错误（第二实例路径）。
// 返回的 release 在进程退出前释放句柄；进程退出时内核也会自动释放，无需常驻持有。
func acquireSingleInstance() (func(), error) {
	h, err := windows.CreateMutex(nil, false, windows.StringToUTF16Ptr(instanceMutexName))
	if err == windows.ERROR_ALREADY_EXISTS {
		return nil, fmt.Errorf("TokenUsage 已在运行")
	}
	if err != nil {
		return nil, fmt.Errorf("创建单实例锁失败: %w", err)
	}
	return func() { _ = windows.CloseHandle(h) }, nil
}

// activateExistingInstance 向已运行实例发送"显示主窗口"信号。
// 命名事件（auto-reset）：第二实例与第一实例看到的是同一个内核对象；
// 若第一实例尚未开始监听（启动早期），事件保持置位，其稍后监听时会立即收到。
// 第二实例发完信号即退出，由第一实例自行唤起窗口。
func activateExistingInstance() {
	ev, err := windows.CreateEvent(nil, 0, 0, windows.StringToUTF16Ptr(showEventName))
	// 注意：事件已存在时 CreateEvent 返回 ERROR_ALREADY_EXISTS，但句柄仍然有效
	//（x/sys 约定：仅当 handle==0 或 lastError==ALREADY_EXISTS 时返回错误，句柄可用）
	if err != nil && err != windows.ERROR_ALREADY_EXISTS {
		log.Printf("[single] create show-event failed: %v", err)
		return
	}
	defer windows.CloseHandle(ev)
	if err := windows.SetEvent(ev); err != nil {
		log.Printf("[single] set show-event failed: %v", err)
	}
}

// watchShowEvent 常驻监听"显示主窗口"信号，收到后唤起主窗口（第二实例双击启动时）。
func watchShowEvent(app *App) {
	ev, err := windows.CreateEvent(nil, 0, 0, windows.StringToUTF16Ptr(showEventName))
	// 事件已存在（ERROR_ALREADY_EXISTS）时句柄仍然有效，正常使用
	if err != nil && err != windows.ERROR_ALREADY_EXISTS {
		log.Printf("[single] watch show-event failed: %v", err)
		return
	}
	defer windows.CloseHandle(ev)
	for {
		rc, err := windows.WaitForSingleObject(ev, windows.INFINITE)
		if err != nil {
			log.Printf("[single] wait show-event failed: %v", err)
			return
		}
		if rc == windows.WAIT_OBJECT_0 {
			log.Println("[single] show-event received, activating window")
			app.showWindow()
		}
	}
}
