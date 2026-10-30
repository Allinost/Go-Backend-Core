package developmentmanager

import (
	"context"
	"strings"
	"time"
)

// validateRequirementReq 校验并归一化需求请求（状态不在此处设置）
func validateRequirementReq(req *CreateRequirementReq) (*Requirement, error) {
	title := strings.TrimSpace(req.Title)
	if l := len([]rune(title)); l < 2 || l > 200 {
		return nil, NewDevelopmentManagerError(ErrInvalidRequirementTitle)
	}
	typ := req.Type
	if typ == "" {
		typ = RequirementTypeFeature
	}
	if !ValidRequirementTypes[typ] {
		return nil, NewDevelopmentManagerError(ErrInvalidRequirementType)
	}
	priority := req.Priority
	if priority == "" {
		priority = PriorityP2
	}
	if !ValidRequirementPriorities[priority] {
		return nil, NewDevelopmentManagerError(ErrInvalidRequirementPriority)
	}
	source := req.Source
	if source == "" {
		source = RequirementSourceManual
	}
	if !ValidRequirementSources[source] {
		return nil, NewDevelopmentManagerError(ErrInvalidRequirementSource)
	}
	return &Requirement{
		Title:    title,
		Content:  req.Content,
		Type:     typ,
		Priority: priority,
		Source:   source,
		Owner:    strings.TrimSpace(req.Owner),
	}, nil
}

// ListRequirements 分页查询项目需求
func (s *Service) ListRequirements(ctx context.Context, projectID int64, req *ListRequirementsReq) ([]Requirement, bool, int, error) {
	if _, err := s.requireProject(ctx, projectID); err != nil {
		return nil, false, 0, err
	}
	if req.Status != "" && !ValidRequirementStatuses[req.Status] {
		return nil, false, 0, NewDevelopmentManagerError(ErrInvalidRequirementStatus)
	}
	if req.Type != "" && !ValidRequirementTypes[req.Type] {
		return nil, false, 0, NewDevelopmentManagerError(ErrInvalidRequirementType)
	}
	if req.Priority != "" && !ValidRequirementPriorities[req.Priority] {
		return nil, false, 0, NewDevelopmentManagerError(ErrInvalidRequirementPriority)
	}
	return s.repo.ListRequirements(ctx, projectID, req)
}

// GetRequirement 查询需求详情
func (s *Service) GetRequirement(ctx context.Context, id int64) (*Requirement, error) {
	return s.repo.GetRequirement(ctx, id)
}

// CreateRequirement 新建需求（自动生成 RQM-xxxx 并写入初始版本）
func (s *Service) CreateRequirement(ctx context.Context, projectID int64, req *CreateRequirementReq, actor string) (*Requirement, error) {
	if _, err := s.requireProject(ctx, projectID); err != nil {
		return nil, err
	}
	r, err := validateRequirementReq(req)
	if err != nil {
		return nil, err
	}
	no, err := s.repo.NextRequirementNo(ctx, projectID)
	if err != nil {
		return nil, err
	}
	r.ProjectID = projectID
	r.Code = formatRequirementCode(no)
	r.Status = RequirementStatusDraft
	r.CreatedBy = actor
	r.UpdatedBy = actor
	changeNote := strings.TrimSpace(req.ChangeNote)
	if changeNote == "" {
		changeNote = "创建需求"
	}
	return s.repo.CreateRequirementTx(ctx, r, changeNote)
}

// UpdateRequirement 更新需求（不改变状态，更新后追加版本快照）
func (s *Service) UpdateRequirement(ctx context.Context, id int64, req *UpdateRequirementReq, actor string) (*Requirement, error) {
	existing, err := s.repo.GetRequirement(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Deleted {
		return nil, NewDevelopmentManagerError(ErrRequirementDeleted)
	}
	updated, err := validateRequirementReq(req)
	if err != nil {
		return nil, err
	}
	updated.ID = existing.ID
	updated.ProjectID = existing.ProjectID
	updated.Code = existing.Code
	updated.Status = existing.Status
	updated.CreatedBy = existing.CreatedBy
	updated.UpdatedBy = actor
	changeNote := strings.TrimSpace(req.ChangeNote)
	if changeNote == "" {
		changeNote = "更新需求"
	}
	return s.repo.UpdateRequirementTx(ctx, updated, existing.Status, changeNote, actor)
}

// ChangeRequirementStatus 需求状态流转（需要 approve 权限）
func (s *Service) ChangeRequirementStatus(ctx context.Context, id int64, req *ChangeRequirementStatusReq, actor string) (*Requirement, error) {
	existing, err := s.repo.GetRequirement(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Deleted {
		return nil, NewDevelopmentManagerError(ErrRequirementDeleted)
	}
	if !ValidRequirementStatuses[req.Status] {
		return nil, NewDevelopmentManagerError(ErrInvalidRequirementStatus)
	}
	if !CanTransition(existing.Status, req.Status) {
		return nil, NewDevelopmentManagerError(ErrInvalidRequirementTransition).WithDetail(
			existing.Status + " -> " + req.Status + " 不在允许的流转路径中")
	}
	changeNote := strings.TrimSpace(req.ChangeNote)
	if changeNote == "" {
		changeNote = "状态流转: " + existing.Status + " -> " + req.Status
	}
	existing.UpdatedBy = actor
	return s.repo.UpdateRequirementTx(ctx, existing, req.Status, changeNote, actor)
}

// DeleteRequirement 软删除需求并写入"已删除"快照
func (s *Service) DeleteRequirement(ctx context.Context, id int64, actor string) error {
	existing, err := s.repo.GetRequirement(ctx, id)
	if err != nil {
		return err
	}
	if existing.Deleted {
		return NewDevelopmentManagerError(ErrRequirementDeleted)
	}
	return s.repo.SoftDeleteRequirementTx(ctx, existing, actor, time.Now())
}

// ListRequirementVersions 查询需求版本历史
func (s *Service) ListRequirementVersions(ctx context.Context, id int64) ([]RequirementVersion, error) {
	if _, err := s.repo.GetRequirement(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.ListRequirementVersions(ctx, id)
}
