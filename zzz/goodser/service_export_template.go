package goodser

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ListExportTemplates 查询导出模板；inventoryID 为空时返回全部。
func (s *Service) ListExportTemplates(ctx context.Context, inventoryID string) ([]ExportTemplate, error) {
	return s.repo.ListExportTemplates(ctx, strings.TrimSpace(inventoryID))
}

// CreateExportTemplate 新建导出模板。
func (s *Service) CreateExportTemplate(ctx context.Context, req *ExportTemplateRequest) (*ExportTemplate, error) {
	name, config, err := normalizeExportTemplate(req)
	if err != nil {
		return nil, err
	}
	inventoryID := strings.TrimSpace(req.InventoryID)
	if req.IsDefault {
		if err := s.repo.ClearDefaultExportTemplates(ctx, inventoryID); err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC()
	tpl := &ExportTemplate{
		ID:        uuid.NewString(),
		Name:      name,
		Config:    config,
		IsDefault: req.IsDefault,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if inventoryID != "" {
		tpl.InventoryID = &inventoryID
	}
	if err := s.repo.CreateExportTemplate(ctx, tpl); err != nil {
		return nil, err
	}
	return tpl, nil
}

// UpdateExportTemplate 全量更新导出模板。
func (s *Service) UpdateExportTemplate(ctx context.Context, id string, req *ExportTemplateRequest) (*ExportTemplate, error) {
	tpl, err := s.repo.GetExportTemplate(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, NewGoodserError(ErrExportTemplateNotFound)
	}
	if err != nil {
		return nil, err
	}
	name, config, err := normalizeExportTemplate(req)
	if err != nil {
		return nil, err
	}
	inventoryID := strings.TrimSpace(req.InventoryID)
	if req.IsDefault {
		if err := s.repo.ClearDefaultExportTemplates(ctx, inventoryID); err != nil {
			return nil, err
		}
	}
	tpl.Name = name
	tpl.Config = config
	tpl.IsDefault = req.IsDefault
	tpl.UpdatedAt = time.Now().UTC()
	if inventoryID == "" {
		tpl.InventoryID = nil
	} else {
		tpl.InventoryID = &inventoryID
	}
	if err := s.repo.UpdateExportTemplate(ctx, tpl); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewGoodserError(ErrExportTemplateNotFound)
		}
		return nil, err
	}
	return tpl, nil
}

// DeleteExportTemplate 删除导出模板。
func (s *Service) DeleteExportTemplate(ctx context.Context, id string) error {
	if err := s.repo.DeleteExportTemplate(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return NewGoodserError(ErrExportTemplateNotFound)
		}
		return err
	}
	return nil
}

func normalizeExportTemplate(req *ExportTemplateRequest) (string, json.RawMessage, error) {
	if req == nil {
		return "", nil, NewGoodserError(ErrExportInvalid, "模板请求为空")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return "", nil, NewGoodserError(ErrExportInvalid, "模板名称不能为空")
	}
	if len([]rune(name)) > 100 {
		return "", nil, NewGoodserError(ErrExportInvalid, "模板名称不能超过 100 字")
	}
	config := req.Config
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	if !json.Valid(config) {
		return "", nil, NewGoodserError(ErrExportInvalid, "模板配置不是合法 JSON")
	}
	var probe map[string]any
	if err := json.Unmarshal(config, &probe); err != nil {
		return "", nil, NewGoodserError(ErrExportInvalid, "模板配置必须是 JSON 对象")
	}
	return name, config, nil
}
