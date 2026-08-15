package main

import "token-dashboard/internal/model"

// ---------------------------------------------------------------------------
// Collection API
// ---------------------------------------------------------------------------

func (a *App) StartCollection() bool {
	return a.collectionSvc.StartCollection()
}

func (a *App) StartFullCollection() bool {
	return a.collectionSvc.StartFullCollection()
}

func (a *App) CollectStatus() *model.CollectStatus {
	return a.collectionSvc.CollectStatus()
}

// CurrentOp 返回当前正在运行的操作名称，供前端展示。返回 "" 表示空闲。
func (a *App) CurrentOp() string {
	return a.collectionSvc.CurrentOp()
}

func (a *App) ClearAllData() error {
	if err := a.collectionSvc.ClearAllData(); err != nil {
		return err
	}
	// 清空数据库后失效仪表盘/会话缓存，否则前端拿到旧缓存数据，表现"清空无效"。
	if a.dashboardSvc != nil {
		a.dashboardSvc.InvalidateCaches()
	}
	return nil
}

// ---------------------------------------------------------------------------
// Auto-Sync API
// ---------------------------------------------------------------------------

func (a *App) SetAutoSyncInterval(minutes int) {
	a.collectionSvc.SetAutoSyncInterval(minutes)
}

func (a *App) GetAutoSyncInterval() int {
	return a.collectionSvc.GetAutoSyncInterval()
}
