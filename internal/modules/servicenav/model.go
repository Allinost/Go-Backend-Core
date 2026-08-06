// Package servicenav 数据模型定义

package servicenav

import "time"

// ServiceType 服务类型，对应 services_nav_type 表
type ServiceType struct {
	ID        string    `json:"_id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Remark    string    `json:"remark,omitempty" db:"remark"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ServiceItem 服务条目（服务导航），对应 services_nav_item 表
// IntranetPort 为 NULL 表示不支持内网访问
type ServiceItem struct {
	ID           string    `json:"_id" db:"id"`
	Name         string    `json:"name" db:"name"`
	Types        []string  `json:"types" db:"types"`
	Protocol     string    `json:"protocol" db:"protocol"`
	Address      string    `json:"address,omitempty" db:"address"`
	IntranetPort *int      `json:"intranet_port" db:"intranet_port"`
	EasyTierPort *int      `json:"easytier_port" db:"easytier_port"`
	Account      string    `json:"account,omitempty" db:"account"`
	PasswordHint string    `json:"password_hint,omitempty" db:"password_hint"`
	Remark       string    `json:"remark,omitempty" db:"remark"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// PaginatedResp 分页响应
type PaginatedResp[T any] struct {
	Items   []T  `json:"items"`
	HasMore bool `json:"has_more"`
	Total   int  `json:"total"`
}

// ---------- 服务类型请求 ----------

// CreateTypeReq 创建服务类型请求
type CreateTypeReq struct {
	Name   string `json:"name" binding:"required"`
	Remark string `json:"remark"`
}

// UpdateTypeReq 更新服务类型请求（指针字段，仅更新非 nil 字段）
type UpdateTypeReq struct {
	Name   *string `json:"name"`
	Remark *string `json:"remark"`
}

// ---------- 服务条目请求 ----------

// CreateItemReq 创建服务条目请求
type CreateItemReq struct {
	Name         string   `json:"name" binding:"required"`
	Types        []string `json:"types" binding:"required"`
	Protocol     string   `json:"protocol"`
	Address      string   `json:"address"`
	IntranetPort *int     `json:"intranet_port"`
	EasyTierPort *int     `json:"easytier_port"`
	Account      string   `json:"account"`
	PasswordHint string   `json:"password_hint"`
	Remark       string   `json:"remark"`
}

// UpdateItemReq 更新服务条目请求（指针字段，仅更新非 nil 字段）
// ClearIntranetPort / ClearEasyTierPort 为 true 时将对应端口置为 NULL
type UpdateItemReq struct {
	Name              *string  `json:"name"`
	Types             []string `json:"types"`
	Protocol          *string  `json:"protocol"`
	Address           *string  `json:"address"`
	IntranetPort      *int     `json:"intranet_port"`
	ClearIntranetPort bool     `json:"clear_intranet_port"`
	EasyTierPort      *int     `json:"easytier_port"`
	ClearEasyTierPort bool     `json:"clear_easytier_port"`
	Account           *string  `json:"account"`
	PasswordHint      *string  `json:"password_hint"`
	Remark            *string  `json:"remark"`
}

// ListItemsReq 服务条目列表查询请求
type ListItemsReq struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	Keyword   string `form:"keyword"`
	Type      string `form:"type"`
	SortBy    string `form:"sort_by"`
	SortOrder string `form:"sort_order"`
}

// Normalize 规范化分页与排序参数
func (r *ListItemsReq) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 20
	}
	if r.SortBy == "" {
		r.SortBy = "name"
	}
	if r.SortOrder == "" {
		r.SortOrder = "asc"
	}
}

// Offset 计算分页偏移量
func (r *ListItemsReq) Offset() int {
	return (r.Page - 1) * r.PageSize
}
