package main

import "token-dashboard/internal/model"

// ---------------------------------------------------------------------------
// Dashboard API
// ---------------------------------------------------------------------------

func (a *App) GetDashboardData() *model.DashboardData {
	return a.dashboardSvc.GetDashboardData()
}

func (a *App) GetHourSeries(days int) []model.HourUsage {
	return a.dashboardSvc.GetHourSeries(days)
}

func (a *App) GetTodayEvents() []model.TimeUsage {
	return a.dashboardSvc.GetTodayEvents()
}

func (a *App) GetModelRanking() []model.ModelRanking {
	return a.dashboardSvc.GetModelRanking()
}

func (a *App) GetModelSeries(modelName string) *model.ModelSeriesData {
	return a.dashboardSvc.GetModelSeries(modelName)
}
