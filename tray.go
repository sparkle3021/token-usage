package main

import (
	"context"
	"log"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// startTray 启动系统托盘（systray 消息泵运行在独立 goroutine，与 Wails 主循环互不干扰）。
// onReady 先等待 app.started 关闭（即 Wails startup 回调完成、ctx 就绪）再创建菜单，
// 保证菜单回调访问 app.ctx 时无数据竞争；startup 尚未完成前用户也无法点击托盘菜单。
func startTray(app *App) {
	go systray.Run(
		func() {
			<-app.started
			log.Println("[tray] ready, creating menu")

			setupTrayIcon() // 平台差异见 platform_*.go
			systray.SetTooltip("TokenUsage - token 用量统计")

			showItem := systray.AddMenuItem("显示主窗口", "显示主窗口")
			collectItem := systray.AddMenuItem("立即采集", "立即采集")
			systray.AddSeparator()
			quitItem := systray.AddMenuItem("退出", "退出 TokenUsage")

			go func() {
				for {
					select {
					case <-showItem.ClickedCh:
						log.Println("[tray] menu: show window")
						app.showWindow()
					case <-collectItem.ClickedCh:
						log.Printf("[tray] menu: collect ok=%v", app.StartCollection())
					case <-quitItem.ClickedCh:
						log.Println("[tray] menu: quit requested")
						app.quitFromTray()
						return
					}
				}
			}()
		},
		func() {
			log.Println("[tray] systray exited")
		},
	)
}

// showWindow 显示并还原主窗口（托盘点击 / 第二实例激活时调用）。
// 从最小化状态恢复时需先 Unminimise 再 Show，否则窗口可能停在任务栏最小化态。
func (a *App) showWindow() {
	if a.ctx == nil {
		log.Println("[tray] showWindow skipped: ctx nil")
		return
	}
	wasMin := runtime.WindowIsMinimised(a.ctx)
	if wasMin {
		runtime.WindowUnminimise(a.ctx)
	}
	runtime.WindowShow(a.ctx)
	log.Printf("[tray] showWindow done wasMinimised=%v", wasMin)
}

// quitFromTray 托盘"退出"：置放行标志后触发 Wails 退出流程。
// 注意：Wails 的 runtime.Quit 也会先调用 OnBeforeClose（见 frontend.go Quit），
// 必须靠 quitting 标志放行；否则 OnBeforeClose 返回 true 会把"退出"也拦成隐藏，
// 应用将永远无法退出。
func (a *App) quitFromTray() {
	if a.ctx == nil {
		return
	}
	a.quitting.Store(true)
	runtime.Quit(a.ctx)
}

// onBeforeClose 窗口关闭拦截：
//   - 点窗口 ×：quitting 未置位 → 隐藏到托盘（返回 true 阻止关闭），应用常驻；
//   - 托盘"退出"：quitting 已置位 → 返回 false 放行，winc.Exit 走 WM_QUIT 正常退出。
func (a *App) onBeforeClose(ctx context.Context) bool {
	if a.quitting.Load() {
		log.Println("[app] closing (tray quit)")
		return false
	}
	log.Println("[app] window close intercepted, hiding to tray")
	runtime.WindowHide(ctx)
	return true
}
