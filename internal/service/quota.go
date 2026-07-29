// Package service 的子文件，用量查询管理服务。
package service

import (
	"fmt"
	"log"
	"sync"
	"time"

	"token-dashboard/internal/collector"
	"token-dashboard/internal/database"
	"token-dashboard/internal/model"
)

// QuotaService 管理用量查询配置的 CRUD 和数据拉取。
type QuotaService struct {
	db *database.Manager
}

func NewQuotaService(db *database.Manager) *QuotaService {
	return &QuotaService{db: db}
}

// ListConfigs 返回所有用量查询配置。
func (s *QuotaService) ListConfigs() []model.QuotaConfig {
	cfgs, err := s.db.ListQuotaConfigs()
	if err != nil {
		log.Printf("[quota] ListConfigs error: %v", err)
		return nil
	}
	return cfgs
}

// GetProviderSchemas 返回所有已注册供应商的 schema。
func (s *QuotaService) GetProviderSchemas() []model.ProviderSchema {
	return collector.ListAllProviderSchemas()
}

// CreateConfig 新建用量查询配置。
func (s *QuotaService) CreateConfig(cfg model.QuotaConfig) (model.QuotaConfig, error) {
	if err := s.db.CreateQuotaConfig(&cfg); err != nil {
		return cfg, fmt.Errorf("创建配置失败: %w", err)
	}
	log.Printf("[quota] CreateConfig ok id=%d provider=%s plan=%s seq=%d", cfg.ID, cfg.Provider, cfg.Plan, cfg.Seq)
	return cfg, nil
}

// UpdateConfig 修改已有配置的别名或 config_json。
func (s *QuotaService) UpdateConfig(cfg model.QuotaConfig) error {
	if err := s.db.UpdateQuotaConfig(&cfg); err != nil {
		return fmt.Errorf("修改配置失败: %w", err)
	}
	log.Printf("[quota] UpdateConfig ok id=%d", cfg.ID)
	return nil
}

// DeleteConfig 删除指定配置。
func (s *QuotaService) DeleteConfig(id int64) error {
	if err := s.db.DeleteQuotaConfig(id); err != nil {
		return fmt.Errorf("删除配置失败: %w", err)
	}
	log.Printf("[quota] DeleteConfig ok id=%d", id)
	return nil
}

// FetchQuota 拉取单个配置的用量数据。
func (s *QuotaService) FetchQuota(id int64) *model.QuotaData {
	cfg, err := s.db.GetQuotaConfigByID(id)
	if err != nil || cfg == nil {
		if err != nil {
			log.Printf("[quota] FetchQuota get config error: %v", err)
		}
		return &model.QuotaData{ConfigID: id, Error: "配置不存在"}
	}

	p := collector.GetQuotaProvider(cfg.Provider)
	if p == nil {
		return &model.QuotaData{ConfigID: id, Error: fmt.Sprintf("未知供应商: %s", cfg.Provider)}
	}

	data, err := p.Fetch(cfg.ConfigJSON)
	if err != nil {
		return &model.QuotaData{ConfigID: id, Provider: cfg.Provider, Plan: cfg.Plan, Name: cfg.DisplayName, Error: err.Error(), FetchedAt: time.Now().Format(time.RFC3339)}
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

// FetchAllQuota 并发拉取所有配置的用量数据。
func (s *QuotaService) FetchAllQuota() []model.QuotaData {
	cfgs := s.ListConfigs()
	if len(cfgs) == 0 {
		return nil
	}

	results := make([]model.QuotaData, len(cfgs))
	var wg sync.WaitGroup
	for i, cfg := range cfgs {
		wg.Add(1)
		go func(idx int, c model.QuotaConfig) {
			defer wg.Done()
			results[idx] = *s.FetchQuota(c.ID)
		}(i, cfg)
	}
	wg.Wait()
	return results
}
