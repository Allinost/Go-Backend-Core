package developmentmanager

import (
	"fmt"
	"time"
)

// 需求类型
const (
	RequirementTypeFeature = "feature" // 新功能
	RequirementTypeChange  = "change"  // 变更
	RequirementTypeFix     = "fix"     // 修复
	RequirementTypeOps     = "ops"     // 运维
)

// 优先级
const (
	PriorityP0 = "P0"
	PriorityP1 = "P1"
	PriorityP2 = "P2"
	PriorityP3 = "P3"
)

// 需求状态
const (
	RequirementStatusDraft      = "draft"      // 草稿
	RequirementStatusReviewing  = "reviewing"  // 评审中
	RequirementStatusApproved   = "approved"   // 已通过
	RequirementStatusPlanned    = "planned"    // 已排期
	RequirementStatusDeveloping = "developing" // 开发中
	RequirementStatusDone       = "done"       // 已完成
	RequirementStatusClosed     = "closed"     // 已关闭
)

// 需求来源
const (
	RequirementSourceManual   = "manual"   // 人工
	RequirementSourceAI       = "ai"       // AI
	RequirementSourceCustomer = "customer" // 客户
)

// VersionStatusDeleted 版本快照专用状态：需求已删除
const VersionStatusDeleted = "deleted"

// ValidRequirementTypes 需求类型合法集合
var ValidRequirementTypes = map[string]bool{
	RequirementTypeFeature: true,
	RequirementTypeChange:  true,
	RequirementTypeFix:     true,
	RequirementTypeOps:     true,
}

// ValidRequirementPriorities 优先级合法集合
var ValidRequirementPriorities = map[string]bool{
	PriorityP0: true, PriorityP1: true, PriorityP2: true, PriorityP3: true,
}

// ValidRequirementStatuses 需求状态合法集合
var ValidRequirementStatuses = map[string]bool{
	RequirementStatusDraft:      true,
	RequirementStatusReviewing:  true,
	RequirementStatusApproved:   true,
	RequirementStatusPlanned:    true,
	RequirementStatusDeveloping: true,
	RequirementStatusDone:       true,
	RequirementStatusClosed:     true,
}

// ValidRequirementSources 需求来源合法集合
var ValidRequirementSources = map[string]bool{
	RequirementSourceManual:   true,
	RequirementSourceAI:       true,
	RequirementSourceCustomer: true,
}

// requirementTransitions 允许的状态流转（key=当前状态，value=可流转到的状态集合）
var requirementTransitions = map[string]map[string]bool{
	RequirementStatusDraft: {
		RequirementStatusReviewing: true,
		RequirementStatusClosed:    true,
	},
	RequirementStatusReviewing: {
		RequirementStatusApproved: true,
		RequirementStatusDraft:    true,
		RequirementStatusClosed:   true,
	},
	RequirementStatusApproved: {
		RequirementStatusPlanned: true,
		RequirementStatusClosed:  true,
	},
	RequirementStatusPlanned: {
		RequirementStatusDeveloping: true,
		RequirementStatusClosed:     true,
	},
	RequirementStatusDeveloping: {
		RequirementStatusDone:   true,
		RequirementStatusClosed: true,
	},
	RequirementStatusDone: {
		RequirementStatusDeveloping: true, // 验收不通过打回
		RequirementStatusClosed:     true,
	},
	RequirementStatusClosed: {},
}

// CanTransition 判断需求状态是否允许流转
func CanTransition(from, to string) bool {
	next, ok := requirementTransitions[from]
	if !ok {
		return false
	}
	return next[to]
}

// Requirement 需求实体
type Requirement struct {
	ID            int64      `json:"id"`
	ProjectID     int64      `json:"project_id"`
	Code          string     `json:"code"`
	Title         string     `json:"title"`
	Content       string     `json:"content"`
	Type          string     `json:"type"`
	Priority      string     `json:"priority"`
	Status        string     `json:"status"`
	Source        string     `json:"source"`
	Owner         string     `json:"owner"`
	LatestVersion int        `json:"latest_version"`
	Deleted       bool       `json:"deleted"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
	CreatedBy     string     `json:"created_by"`
	UpdatedBy     string     `json:"updated_by"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// RequirementVersion 需求版本快照
type RequirementVersion struct {
	ID            int64     `json:"id"`
	RequirementID int64     `json:"requirement_id"`
	VersionNo     int       `json:"version_no"`
	Title         string    `json:"title"`
	Content       string    `json:"content"`
	Status        string    `json:"status"`
	ChangeNote    string    `json:"change_note"`
	Operator      string    `json:"operator"`
	CreatedAt     time.Time `json:"created_at"`
}

// ---------- 请求结构 ----------

// ListRequirementsReq 需求列表查询参数
type ListRequirementsReq struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Status   string `form:"status"`
	Type     string `form:"type"`
	Priority string `form:"priority"`
	Keyword  string `form:"keyword"`
	Deleted  *bool  `form:"deleted"`
}

// Normalize 分页参数归一化
func (r *ListRequirementsReq) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 20
	}
}

// Offset 计算 SQL OFFSET
func (r *ListRequirementsReq) Offset() int { return (r.Page - 1) * r.PageSize }

// CreateRequirementReq 新建需求请求
type CreateRequirementReq struct {
	Title      string `json:"title" binding:"required"`
	Content    string `json:"content"`
	Type       string `json:"type"`
	Priority   string `json:"priority"`
	Source     string `json:"source"`
	Owner      string `json:"owner"`
	ChangeNote string `json:"change_note"`
}

// UpdateRequirementReq 更新需求请求（不含状态，状态走流转接口）
type UpdateRequirementReq = CreateRequirementReq

// ChangeRequirementStatusReq 需求状态流转请求
type ChangeRequirementStatusReq struct {
	Status     string `json:"status" binding:"required"`
	ChangeNote string `json:"change_note"`
}

// formatRequirementCode 生成项目内需求编号（RQM-0001）
func formatRequirementCode(n int) string {
	return fmt.Sprintf("RQM-%04d", n)
}
