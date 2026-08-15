package main

// Version 应用版本号。
// 发布时须与 wails.json 的 productVersion 同步更新（exe 版本资源使用）。
const Version = "0.5.0"

// GetAppVersion 返回当前应用版本号，供前端设置页展示。
func (a *App) GetAppVersion() string {
	return Version
}
