// Package servicenav 业务逻辑层 — 服务类型与服务条目 CRUD

package servicenav

import (
	"context"
	"database/sql"
	"strings"
)

// Service 服务导航业务逻辑
type Service struct {
	repo *Repository
}

// NewService 创建 Service
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ---------- 服务类型 ----------

// ListTypes 查询全部服务类型
func (s *Service) ListTypes(ctx context.Context) ([]ServiceType, error) {
	return s.repo.ListTypes(ctx)
}

// GetType 获取服务类型详情
func (s *Service) GetType(ctx context.Context, id string) (*ServiceType, error) {
	t, err := s.repo.GetType(ctx, id)
	if err == sql.ErrNoRows {
		return nil, NewNavError(ErrTypeNotFound)
	}
	return t, err
}

// CreateType 创建服务类型（校验名称唯一）
func (s *Service) CreateType(ctx context.Context, req *CreateTypeReq) (*ServiceType, error) {
	if req.Name == "" {
		return nil, NewNavError(ErrParamNameRequired)
	}
	exists, err := s.repo.TypeNameExists(ctx, req.Name, "")
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, NewNavError(ErrTypeNameConflict)
	}
	return s.repo.CreateType(ctx, req)
}

// UpdateType 更新服务类型（校验名称唯一）
func (s *Service) UpdateType(ctx context.Context, id string, req *UpdateTypeReq) (*ServiceType, error) {
	if req.Name != nil {
		exists, err := s.repo.TypeNameExists(ctx, *req.Name, id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, NewNavError(ErrTypeNameConflict)
		}
	}
	t, err := s.repo.UpdateType(ctx, id, req)
	if err == sql.ErrNoRows {
		return nil, NewNavError(ErrTypeNotFound)
	}
	return t, err
}

// DeleteType 删除服务类型（被服务条目引用时禁止删除）
func (s *Service) DeleteType(ctx context.Context, id string) error {
	t, err := s.repo.GetType(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return NewNavError(ErrTypeNotFound)
		}
		return err
	}
	count, err := s.repo.CountItemsByType(ctx, t.Name)
	if err != nil {
		return err
	}
	if count > 0 {
		return NewNavError(ErrTypeInUse)
	}
	return s.repo.DeleteType(ctx, id)
}

// ---------- 服务条目 ----------

// ListItems 分页查询服务条目
func (s *Service) ListItems(ctx context.Context, req *ListItemsReq) ([]ServiceItem, bool, int, error) {
	req.Normalize()
	return s.repo.ListItems(ctx, req)
}

// GetItem 获取服务条目详情
func (s *Service) GetItem(ctx context.Context, id string) (*ServiceItem, error) {
	item, err := s.repo.GetItem(ctx, id)
	if err == sql.ErrNoRows {
		return nil, NewNavError(ErrItemNotFound)
	}
	return item, err
}

// CreateItem 创建服务条目（校验名称唯一 + 类型存在）
func (s *Service) CreateItem(ctx context.Context, req *CreateItemReq) (*ServiceItem, error) {
	if req.Name == "" {
		return nil, NewNavError(ErrParamNameRequired)
	}
	exists, err := s.repo.GetItemByName(ctx, req.Name)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if exists != nil {
		return nil, NewNavError(ErrItemNameConflict)
	}
	if err := s.validateItemTypes(ctx, req.Types); err != nil {
		return nil, err
	}
	return s.repo.CreateItem(ctx, req)
}

// UpdateItem 更新服务条目（校验名称唯一 + 类型存在）
func (s *Service) UpdateItem(ctx context.Context, id string, req *UpdateItemReq) (*ServiceItem, error) {
	cur, err := s.repo.GetItem(ctx, id)
	if err == sql.ErrNoRows {
		return nil, NewNavError(ErrItemNotFound)
	}
	if err != nil {
		return nil, err
	}

	newName := cur.Name
	if req.Name != nil {
		newName = *req.Name
	}
	if newName != cur.Name {
		dup, err := s.repo.GetItemByName(ctx, newName)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if dup != nil {
			return nil, NewNavError(ErrItemNameConflict)
		}
	}

	types := cur.Types
	if req.Types != nil {
		types = req.Types
	}
	if err := s.validateItemTypes(ctx, types); err != nil {
		return nil, err
	}

	return s.repo.UpdateItem(ctx, id, req)
}

// DeleteItem 删除服务条目
func (s *Service) DeleteItem(ctx context.Context, id string) error {
	if _, err := s.repo.GetItem(ctx, id); err != nil {
		if err == sql.ErrNoRows {
			return NewNavError(ErrItemNotFound)
		}
		return err
	}
	return s.repo.DeleteItem(ctx, id)
}

// validateItemTypes 校验条目引用的类型名称均已在类型表中注册
func (s *Service) validateItemTypes(ctx context.Context, types []string) error {
	if len(types) == 0 {
		return nil
	}
	missing, err := s.repo.ValidateTypes(ctx, types)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return NewNavError(ErrInvalidTypeRef, strings.Join(missing, ","))
	}
	return nil
}
