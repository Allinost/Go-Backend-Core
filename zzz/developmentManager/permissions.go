package developmentmanager

import (
	"fmt"

	"github.com/Allinost/go-backend-core/internal/services/auth"
)

// PermSpec 权限点规格（资源 × 动作）
type PermSpec struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Name     string `json:"name"`
	Desc     string `json:"desc"`
}

// PermissionSpecs 本模块权限矩阵（规划 §4.1，名称 = resource.action 全局唯一）
var PermissionSpecs = []PermSpec{
	// 项目台账
	{Resource: "dm.project", Action: "read", Desc: "查看项目台账"},
	{Resource: "dm.project", Action: "create", Desc: "新建项目"},
	{Resource: "dm.project", Action: "update", Desc: "修改项目"},
	{Resource: "dm.project", Action: "delete", Desc: "删除项目（软删除，级联标记关联）"},
	{Resource: "dm.project", Action: "restore", Desc: "恢复软删除的项目"},
	// 项目关联
	{Resource: "dm.relation", Action: "read", Desc: "查看项目关联"},
	{Resource: "dm.relation", Action: "create", Desc: "新建项目关联"},
	{Resource: "dm.relation", Action: "delete", Desc: "删除项目关联"},
	// 部署资产
	{Resource: "dm.deployment", Action: "read", Desc: "查看部署资产（容器/接入点/白名单/资源/卷/备份）"},
	{Resource: "dm.deployment", Action: "create", Desc: "新建部署资产"},
	{Resource: "dm.deployment", Action: "update", Desc: "修改部署资产"},
	{Resource: "dm.deployment", Action: "delete", Desc: "删除部署资产"},
	// 数据字典
	{Resource: "dm.dictionary", Action: "read", Desc: "查看数据表字典"},
	{Resource: "dm.dictionary", Action: "create", Desc: "登记数据表/字段"},
	{Resource: "dm.dictionary", Action: "update", Desc: "修改数据表/字段"},
	{Resource: "dm.dictionary", Action: "delete", Desc: "删除数据表/字段"},
	{Resource: "dm.dictionary", Action: "scan", Desc: "从数据库扫描导入字典初稿"},
	// 需求
	{Resource: "dm.requirement", Action: "read", Desc: "查看需求"},
	{Resource: "dm.requirement", Action: "create", Desc: "新建需求"},
	{Resource: "dm.requirement", Action: "update", Desc: "修改需求（产生版本快照）"},
	{Resource: "dm.requirement", Action: "delete", Desc: "删除需求（软删除 + 快照）"},
	{Resource: "dm.requirement", Action: "approve", Desc: "需求状态流转/评审通过"},
	// 生成任务
	{Resource: "dm.generation", Action: "read", Desc: "查看生成任务"},
	{Resource: "dm.generation", Action: "execute", Desc: "触发生成方案/代码/文档"},
	{Resource: "dm.generation", Action: "autocommit", Desc: "允许自动提交生成产物（auto_commit 开关的前置权限）"},
	// 资产索引
	{Resource: "dm.asset", Action: "read", Desc: "查看归类资产索引"},
	{Resource: "dm.asset", Action: "create", Desc: "登记归类资产"},
	{Resource: "dm.asset", Action: "delete", Desc: "删除归类资产"},
}

func init() {
	for i := range PermissionSpecs {
		PermissionSpecs[i].Name = PermissionSpecs[i].Resource + "." + PermissionSpecs[i].Action
	}
}

// editorActions 编辑者可用动作（无删除/审批/扫描/执行/自动提交）
var editorActions = map[string]bool{"read": true, "create": true, "update": true}

// SeedPermissions 幂等播种权限点与默认角色（editor/viewer），已存在的权限与已配置的角色不覆盖
func SeedPermissions(rbac *auth.RBACService) error {
	if rbac == nil {
		return fmt.Errorf("developmentmanager: RBAC 服务不可用")
	}

	perms, err := rbac.ListPermissions()
	if err != nil {
		return fmt.Errorf("developmentmanager: 读取权限列表失败: %w", err)
	}
	existing := make(map[string]auth.Permission, len(perms))
	for _, p := range perms {
		existing[p.Name] = p
	}

	created := 0
	for _, spec := range PermissionSpecs {
		if _, ok := existing[spec.Name]; ok {
			continue
		}
		if _, err := rbac.CreatePermission(spec.Name, spec.Resource, spec.Action); err != nil {
			return fmt.Errorf("developmentmanager: 播种权限 %s 失败: %w", spec.Name, err)
		}
		created++
	}
	if created > 0 {
		perms, err = rbac.ListPermissions()
		if err != nil {
			return fmt.Errorf("developmentmanager: 读取权限列表失败: %w", err)
		}
		for _, p := range perms {
			existing[p.Name] = p
		}
	}

	// 默认角色：editor = 各资源 read/create/update + 需求审批，viewer = 各资源 read
	if err := ensureRole(rbac, existing, "editor", func(spec PermSpec) bool {
		return editorActions[spec.Action] || spec.Name == "dm.requirement.approve"
	}); err != nil {
		return err
	}
	if err := ensureRole(rbac, existing, "viewer", func(spec PermSpec) bool {
		return spec.Action == "read"
	}); err != nil {
		return err
	}
	return nil
}

// ensureRole 确保角色存在（不存在则创建并按 filter 分配权限，已存在则不覆盖）
func ensureRole(rbac *auth.RBACService, existing map[string]auth.Permission, role string, filter func(PermSpec) bool) error {
	roles, err := rbac.ListRoles()
	if err != nil {
		return fmt.Errorf("developmentmanager: 读取角色列表失败: %w", err)
	}
	for _, r := range roles {
		if r.Name == role {
			return nil // 已存在，保留管理员现有配置
		}
	}
	if err := rbac.CreateRole(role); err != nil {
		return fmt.Errorf("developmentmanager: 创建角色 %s 失败: %w", role, err)
	}
	var ids []uint
	for _, spec := range PermissionSpecs {
		if !filter(spec) {
			continue
		}
		if p, ok := existing[spec.Name]; ok {
			ids = append(ids, p.ID)
		}
	}
	if err := rbac.SetRolePermissions(role, ids); err != nil {
		return fmt.Errorf("developmentmanager: 分配角色 %s 权限失败: %w", role, err)
	}
	return nil
}
