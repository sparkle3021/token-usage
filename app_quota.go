package main

import "token-dashboard/internal/model"

// ---------------------------------------------------------------------------
// Quota API
// ---------------------------------------------------------------------------

func (a *App) ListQuotaConfigs() []model.QuotaConfig {
	return a.quotaSvc.List()
}

func (a *App) GetProviderSchemas() []model.ProviderSchema {
	return a.quotaSvc.GetProviderSchemas()
}

func (a *App) CreateQuotaConfig(cfg model.QuotaConfig) model.QuotaConfig {
	created, err := a.quotaSvc.Create(cfg)
	if err != nil {
		return model.QuotaConfig{ConfigJSON: err.Error()}
	}
	return created
}

func (a *App) UpdateQuotaConfig(cfg model.QuotaConfig) error {
	return a.quotaSvc.Update(cfg)
}

func (a *App) DeleteQuotaConfig(id int64) error {
	return a.quotaSvc.Delete(id)
}

func (a *App) FetchQuota(id int64) *model.QuotaData {
	return a.quotaSvc.Fetch(id)
}

func (a *App) FetchAllQuota() []model.QuotaData {
	return a.quotaSvc.FetchAll()
}
