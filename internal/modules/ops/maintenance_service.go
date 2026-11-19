package ops

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Allinost/go-backend-core/internal/services/auth"
)

var (
	ErrMaintenanceActive       = errors.New("已有进行中的维护")
	ErrMaintenanceNotFound     = errors.New("维护记录不存在")
	ErrInvalidMaintenanceMode  = errors.New("无效的维护模式")
	ErrMaintenanceSyncRequired = errors.New("开始迁移维护前必须确认 L1→L2 已同步")
	ErrMaintenanceSyncBack     = errors.New("结束迁回维护前必须确认 L2→L1 已反向同步")
)

// MaintenanceService L1/L2 手动维护状态机。
type MaintenanceService struct {
	repo *MaintenanceRepository
}

func NewMaintenanceService(repo *MaintenanceRepository) *MaintenanceService {
	return &MaintenanceService{repo: repo}
}

// Status 当前维护状态。
func (s *MaintenanceService) Status(ctx context.Context) (*MaintenanceStatusResp, error) {
	active, err := s.repo.Active(ctx)
	if err != nil {
		return nil, err
	}
	resp := &MaintenanceStatusResp{
		Active: active,
		Node:   "L1",
		Modes:  []string{MaintenanceModeShutdown, MaintenanceModeMigrateReturn, MaintenanceModeMigrateNoReturn},
	}
	if active != nil {
		if active.TargetNode == "L2" && active.SwitchedToL2 {
			resp.Node = "L2"
		}
		if active.Mode != MaintenanceModeShutdown && !active.SyncConfirmed {
			resp.Warning = "未确认 L1→L2 同步，禁止切换"
		}
	}
	return resp, nil
}

// Start 开始维护。migrate_* 必须先确认 L1→L2 同步。
func (s *MaintenanceService) Start(ctx context.Context, req *StartMaintenanceReq, operator string) (*Maintenance, error) {
	mode := strings.TrimSpace(req.Mode)
	if !ValidMaintenanceModes[mode] {
		return nil, ErrInvalidMaintenanceMode
	}
	active, err := s.repo.Active(ctx)
	if err != nil {
		return nil, err
	}
	if active != nil {
		return nil, ErrMaintenanceActive
	}
	if mode != MaintenanceModeShutdown && !req.SyncConfirmed {
		return nil, ErrMaintenanceSyncRequired
	}
	m := &Maintenance{
		Mode:              mode,
		Status:            MaintenanceStatusInProgress,
		SourceNode:        "L1",
		TargetNode:        "L1",
		SwitchedToL2:      false,
		SyncConfirmed:     req.SyncConfirmed,
		SyncBackConfirmed: false,
		Note:              strings.TrimSpace(req.Note),
		StartedBy:         operator,
	}
	if mode != MaintenanceModeShutdown {
		m.TargetNode = "L2"
	}
	return s.repo.Start(ctx, m)
}

// End 结束维护。migrate_return 且确实切过 L2 时，必须先确认 L2→L1 反向同步。
func (s *MaintenanceService) End(ctx context.Context, req *EndMaintenanceReq, operator string) (*Maintenance, error) {
	active, err := s.repo.Active(ctx)
	if err != nil {
		return nil, err
	}
	if active == nil {
		return nil, ErrMaintenanceNotFound
	}

	switched := false
	if active.Mode != MaintenanceModeShutdown {
		if req.SwitchedToL2 != nil {
			switched = *req.SwitchedToL2
		} else {
			switched = true
		}
	}
	if switched && active.Mode == MaintenanceModeMigrateReturn && !req.SyncBackConfirmed {
		return nil, ErrMaintenanceSyncBack
	}
	targetNode := "L1"
	if switched {
		targetNode = "L2"
	}
	return s.repo.End(ctx, active.ID, switched, req.SyncBackConfirmed, targetNode, operator, strings.TrimSpace(req.Note))
}

func (s *MaintenanceService) History(ctx context.Context, limit int) ([]Maintenance, error) {
	return s.repo.List(ctx, limit)
}

// SeedMaintenancePermissions 幂等登记维护权限点。
func SeedMaintenancePermissions(rbac *auth.RBACService) error {
	if rbac == nil {
		return fmt.Errorf("ops: RBAC 不可用")
	}
	existing, err := rbac.ListPermissions()
	if err != nil {
		return fmt.Errorf("ops: 读取权限失败: %w", err)
	}
	have := make(map[string]bool, len(existing))
	for _, p := range existing {
		have[p.Name] = true
	}
	specs := map[string]string{
		"ops.maintenance.read":  "查看维护状态/历史",
		"ops.maintenance.start": "开始维护",
		"ops.maintenance.end":   "结束维护",
	}
	for name := range specs {
		if have[name] {
			continue
		}
		action := strings.TrimPrefix(name, "ops.maintenance.")
		if _, err := rbac.CreatePermission(name, "ops.maintenance", action); err != nil {
			return fmt.Errorf("ops: 创建权限 %s 失败: %w", name, err)
		}
	}
	return nil
}
