package main

import "token-dashboard/internal/model"

// ---------------------------------------------------------------------------
// Device API
// ---------------------------------------------------------------------------

func (a *App) GetDevices() []model.DeviceInfo {
	return a.deviceSvc.ListDevices()
}

func (a *App) RenameDevice(deviceID, displayName string) error {
	return a.deviceSvc.RenameDevice(deviceID, displayName)
}
