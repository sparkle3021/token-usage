package main

import "token-dashboard/internal/model"

// ---------------------------------------------------------------------------
// Settings API
// ---------------------------------------------------------------------------

func (a *App) GetSettings() model.AppConfig {
	return a.settingSvc.GetSettings()
}

func (a *App) SaveSettings(cfg model.AppConfig) error {
	return a.settingSvc.SaveSettings(cfg)
}
