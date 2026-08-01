// Package quota 用量查询域：供应商注册、配置 CRUD 与余额/配额拉取。
package quota

import (
	"fmt"
	"log"
	"sync"
	"time"

	"token-dashboard/internal/database"
	"token-dashboard/internal/model"
)

// Service 管理用量查询配置的 CRUD 和数据拉取。
type Service struct {
	db *database.Manager
}

// NewService 创建用量查询服务实例。
func NewService(db *database.Manager) *Service {
	return &Service{db: db}
}

// List 返回所有用量查询配置。
func (s *Service) List() []model.QuotaConfig {
	cfgs, err := s.ListConfigs()
	if err != nil {
		log.Printf("[quota] List error: %v", err)
		return nil
	}
	return cfgs
}

// GetProviderSchemas 返回所有已注册供应商的 schema。
func (s *Service) GetProviderSchemas() []model.ProviderSchema {
	return ListAllProviderSchemas()
}

// Create 新建用量查询配置。
func (s *Service) Create(cfg model.QuotaConfig) (model.QuotaConfig, error) {
	if err := s.CreateConfig(&cfg); err != nil {
		return cfg, fmt.Errorf("创建配置失败: %w", err)
	}
	log.Printf("[quota] Create ok id=%d provider=%s plan=%s seq=%d", cfg.ID, cfg.Provider, cfg.Plan, cfg.Seq)
	return cfg, nil
}

// Update 修改已有配置的别名或 config_json，变更后重置有效性。
func (s *Service) Update(cfg model.QuotaConfig) error {
	if err := s.UpdateConfig(&cfg); err != nil {
		return fmt.Errorf("修改配置失败: %w", err)
	}
	s.SetConfigValid(cfg.ID, true)
	log.Printf("[quota] Update ok id=%d", cfg.ID)
	return nil
}

// Delete 删除指定配置。
func (s *Service) Delete(id int64) error {
	if err := s.DeleteConfig(id); err != nil {
		return fmt.Errorf("删除配置失败: %w", err)
	}
	log.Printf("[quota] Delete ok id=%d", id)
	return nil
}

// Fetch 拉取单个配置的用量数据。
func (s *Service) Fetch(id int64) *model.QuotaData {
	cfg, err := s.GetConfigByID(id)
	if err != nil || cfg == nil {
		if err != nil {
			log.Printf("[quota] Fetch get config error: %v", err)
		}
		return &model.QuotaData{ConfigID: id, Error: "配置不存在"}
	}

	p := GetQuotaProvider(cfg.Provider)
	if p == nil {
		return &model.QuotaData{ConfigID: id, Error: fmt.Sprintf("未知供应商: %s", cfg.Provider)}
	}

	data, err := p.Fetch(cfg.ConfigJSON)
	if err != nil {
		s.SetConfigValid(id, false)
		log.Printf("[quota] Fetch error id=%d err=%v", id, err)
		return &model.QuotaData{ConfigID: id, Provider: cfg.Provider, Plan: cfg.Plan, Name: cfg.DisplayName, Error: err.Error(), FetchedAt: time.Now().Format(time.RFC3339)}
	}

	// 请求成功，标记为有效
	if !cfg.IsValid {
		s.SetConfigValid(id, true)
	}

	data.ConfigID = id
	data.Provider = cfg.Provider
	data.Plan = cfg.Plan
	if cfg.DisplayName != "" {
		data.Name = cfg.DisplayName
	} else {
		data.Name = fmt.Sprintf("%d", cfg.Seq)
	}
	data.FetchedAt = time.Now().Format(time.RFC3339)
	return data
}

// FetchAll 并发拉取所有配置的用量数据。
func (s *Service) FetchAll() []model.QuotaData {
	cfgs := s.List()
	if len(cfgs) == 0 {
		return nil
	}

	results := make([]model.QuotaData, len(cfgs))
	var wg sync.WaitGroup
	for i, cfg := range cfgs {
		wg.Add(1)
		go func(idx int, c model.QuotaConfig) {
			defer wg.Done()
			results[idx] = *s.Fetch(c.ID)
		}(i, cfg)
	}
	wg.Wait()
	return results
}
