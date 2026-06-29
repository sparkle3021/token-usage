package main

import (
	"context"

	"token-dashboard/internal/service"
)

type App struct {
	ctx     context.Context
	service *service.Service
}

func NewApp() *App {
	return &App{
		service: service.New(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}
