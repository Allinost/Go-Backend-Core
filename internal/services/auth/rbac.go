package auth

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Role 用户角色类型
type Role string

const (
	RoleAdmin Role = "admin" // 管理员角色
	RoleUser  Role = "user"  // 普通用户角色
)

// Permission 权限定义，包含资源与操作
type Permission struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string    `json:"name" gorm:"uniqueIndex;size:255"`
	Resource  string    `json:"resource" gorm:"size:100"`
	Action    string    `json:"action" gorm:"size:50"`
	CreatedAt time.Time `json:"created_at"`
}

// RolePermission 角色与权限的关联表
type RolePermission struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Role         string    `json:"role" gorm:"size:50;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	PermissionID uint      `json:"permission_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// RoleModel 角色模型
type RoleModel struct {
	Name      string    `json:"name" gorm:"primaryKey;size:50"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserRole 用户与角色的关联表
type UserRole struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID    uint      `json:"user_id" gorm:"index"`
	Role      string    `json:"role" gorm:"size:50"`
	CreatedAt time.Time `json:"created_at"`
}

var defaultPermissions = map[string][]struct {
	Resource string
	Action   string
}{
	"admin": {
		{"*", "*"},
	},
	"user": {
		{"task", "read"},
		{"task", "create"},
		{"task", "update"},
		{"task", "delete"},
		{"user", "read"},
		{"user", "update"},
	},
}

// RBACStore 基于角色的访问控制存储接口
type RBACStore interface {
	EnsureDefaultPermissions() error
	AssignRole(userID uint, role string) error
	GetRoles(userID uint) ([]string, error)
	GetPermissions(roles []string) ([]Permission, error)

	// Permission CRUD
	CreatePermission(name, resource, action string) (*Permission, error)
	ListPermissions() ([]Permission, error)
	UpdatePermission(id uint, name, resource, action string) error
	DeletePermission(id uint) error
	GetPermissionByID(id uint) (*Permission, error)

	// Role CRUD
	CreateRole(name string) error
	ListRoles() ([]RoleModel, error)
	UpdateRole(oldName, newName string) error
	DeleteRole(name string) error

	// Role-Permission binding
	SetRolePermissions(role string, permissionIDs []uint) error
	GetRolePermissions(role string) ([]Permission, error)

	// User management
	GetUserPermissions(userID uint) ([]Permission, error)
	RemoveUserRole(userID uint, role string) error
}

// InMemoryRBACStore 基于内存的 RBAC 存储实现
type InMemoryRBACStore struct {
	mu        sync.RWMutex
	allPerms  []Permission
	permSeq   uint
	rolePerms map[string][]uint // role -> permission IDs
	userRoles map[uint][]string
	roles     map[string]bool
}

// NewInMemoryRBACStore 创建内存 RBAC 存储实例
func NewInMemoryRBACStore() *InMemoryRBACStore {
	s := &InMemoryRBACStore{
		rolePerms: make(map[string][]uint),
		userRoles: make(map[uint][]string),
		roles:     make(map[string]bool),
	}
	s.roles[string(RoleAdmin)] = true
	s.roles[string(RoleUser)] = true
	return s
}

// EnsureDefaultPermissions 初始化默认角色权限
func (s *InMemoryRBACStore) EnsureDefaultPermissions() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for role, perms := range defaultPermissions {
		for _, p := range perms {
			s.permSeq++
			perm := Permission{
				ID:       s.permSeq,
				Name:     fmt.Sprintf("%s:%s", p.Resource, p.Action),
				Resource: p.Resource,
				Action:   p.Action,
			}
			s.allPerms = append(s.allPerms, perm)
			s.rolePerms[role] = append(s.rolePerms[role], perm.ID)
		}
	}
	return nil
}

// AssignRole 为用户分配角色
func (s *InMemoryRBACStore) AssignRole(userID uint, role string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.roles[role] {
		return fmt.Errorf("auth: 无效角色 %s", role)
	}
	s.userRoles[userID] = []string{role}
	return nil
}

// GetRoles 获取用户的角色列表
func (s *InMemoryRBACStore) GetRoles(userID uint) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	roles := s.userRoles[userID]
	if len(roles) == 0 {
		return []string{string(RoleUser)}, nil
	}
	return roles, nil
}

// GetPermissions 获取指定角色列表的权限集合（已去重）
func (s *InMemoryRBACStore) GetPermissions(roles []string) ([]Permission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Permission
	seen := make(map[string]bool)
	for _, role := range roles {
		for _, pid := range s.rolePerms[role] {
			perm := s.findPermByID(pid)
			if perm != nil && !seen[perm.Name] {
				result = append(result, *perm)
				seen[perm.Name] = true
			}
		}
	}
	return result, nil
}

func (s *InMemoryRBACStore) findPermByID(id uint) *Permission {
	for i := range s.allPerms {
		if s.allPerms[i].ID == id {
			return &s.allPerms[i]
		}
	}
	return nil
}

// CreatePermission 创建权限
func (s *InMemoryRBACStore) CreatePermission(name, resource, action string) (*Permission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.allPerms {
		if s.allPerms[i].Name == name {
			return nil, fmt.Errorf("auth: 权限 %s 已存在", name)
		}
	}

	s.permSeq++
	perm := Permission{
		ID:        s.permSeq,
		Name:      name,
		Resource:  resource,
		Action:    action,
		CreatedAt: time.Now(),
	}
	s.allPerms = append(s.allPerms, perm)
	return &s.allPerms[len(s.allPerms)-1], nil
}

// ListPermissions 列出所有权限
func (s *InMemoryRBACStore) ListPermissions() ([]Permission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Permission, len(s.allPerms))
	copy(result, s.allPerms)
	return result, nil
}

// UpdatePermission 更新权限
func (s *InMemoryRBACStore) UpdatePermission(id uint, name, resource, action string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.allPerms {
		if s.allPerms[i].ID == id {
			s.allPerms[i].Name = name
			s.allPerms[i].Resource = resource
			s.allPerms[i].Action = action
			return nil
		}
	}
	return fmt.Errorf("auth: 权限 %d 不存在", id)
}

// DeletePermission 删除权限
func (s *InMemoryRBACStore) DeletePermission(id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.allPerms {
		if s.allPerms[i].ID == id {
			s.allPerms = append(s.allPerms[:i], s.allPerms[i+1:]...)
			// Clean up role-permission associations
			for role, pids := range s.rolePerms {
				for j, pid := range pids {
					if pid == id {
						s.rolePerms[role] = append(pids[:j], pids[j+1:]...)
						break
					}
				}
			}
			return nil
		}
	}
	return fmt.Errorf("auth: 权限 %d 不存在", id)
}

// GetPermissionByID 根据ID获取权限
func (s *InMemoryRBACStore) GetPermissionByID(id uint) (*Permission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	perm := s.findPermByID(id)
	if perm == nil {
		return nil, fmt.Errorf("auth: 权限 %d 不存在", id)
	}
	return perm, nil
}

// CreateRole 创建角色
func (s *InMemoryRBACStore) CreateRole(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.roles[name] {
		return fmt.Errorf("auth: 角色 %s 已存在", name)
	}
	s.roles[name] = true
	return nil
}

// ListRoles 列出所有角色
func (s *InMemoryRBACStore) ListRoles() ([]RoleModel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []RoleModel
	for name := range s.roles {
		result = append(result, RoleModel{Name: name})
	}
	return result, nil
}

// UpdateRole 更新角色名称
func (s *InMemoryRBACStore) UpdateRole(oldName, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.roles[oldName] {
		return fmt.Errorf("auth: 角色 %s 不存在", oldName)
	}
	if s.roles[newName] {
		return fmt.Errorf("auth: 角色 %s 已存在", newName)
	}
	delete(s.roles, oldName)
	s.roles[newName] = true
	s.rolePerms[newName] = s.rolePerms[oldName]
	delete(s.rolePerms, oldName)
	return nil
}

// DeleteRole 删除角色
func (s *InMemoryRBACStore) DeleteRole(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == string(RoleAdmin) {
		return fmt.Errorf("auth: 不能删除管理员角色")
	}
	if !s.roles[name] {
		return fmt.Errorf("auth: 角色 %s 不存在", name)
	}
	delete(s.roles, name)
	delete(s.rolePerms, name)
	return nil
}

// SetRolePermissions 设置角色的权限列表
func (s *InMemoryRBACStore) SetRolePermissions(role string, permissionIDs []uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.roles[role] {
		return fmt.Errorf("auth: 角色 %s 不存在", role)
	}
	// Validate all permission IDs exist
	for _, pid := range permissionIDs {
		if s.findPermByID(pid) == nil {
			return fmt.Errorf("auth: 权限 %d 不存在", pid)
		}
	}
	s.rolePerms[role] = permissionIDs
	return nil
}

// GetRolePermissions 获取角色的权限列表
func (s *InMemoryRBACStore) GetRolePermissions(role string) ([]Permission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.roles[role] {
		return nil, fmt.Errorf("auth: 角色 %s 不存在", role)
	}
	var result []Permission
	for _, pid := range s.rolePerms[role] {
		perm := s.findPermByID(pid)
		if perm != nil {
			result = append(result, *perm)
		}
	}
	return result, nil
}

// GetUserPermissions 获取用户的所有权限（通过角色）
func (s *InMemoryRBACStore) GetUserPermissions(userID uint) ([]Permission, error) {
	roles, err := s.GetRoles(userID)
	if err != nil {
		return nil, err
	}
	return s.GetPermissions(roles)
}

// RemoveUserRole 移除用户的某个角色
func (s *InMemoryRBACStore) RemoveUserRole(userID uint, role string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	roles := s.userRoles[userID]
	for i, r := range roles {
		if r == role {
			s.userRoles[userID] = append(roles[:i], roles[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("auth: 用户 %d 没有角色 %s", userID, role)
}

// MySQLRBACStore 基于 MySQL 的 RBAC 存储实现
type MySQLRBACStore struct {
	db *gorm.DB
}

// NewMySQLRBACStore 使用现有 sql.DB 创建 MySQL RBAC 存储
func NewMySQLRBACStore(sqlDB *sql.DB) (*MySQLRBACStore, error) {
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("auth: RBAC GORM 初始化失败: %w", err)
	}
	return &MySQLRBACStore{db: db}, nil
}

// AutoMigrate 自动迁移 RBAC 相关数据表
func (s *MySQLRBACStore) AutoMigrate() error {
	return s.db.AutoMigrate(&RoleModel{}, &Permission{}, &RolePermission{}, &UserRole{})
}

// EnsureDefaultPermissions 初始化默认角色权限（MySQL 实现）
func (s *MySQLRBACStore) EnsureDefaultPermissions() error {
	// Ensure default roles exist
	for _, role := range []string{string(RoleAdmin), string(RoleUser)} {
		s.db.Where("name = ?", role).FirstOrCreate(&RoleModel{Name: role})
	}

	for role, perms := range defaultPermissions {
		for _, p := range perms {
			perm := Permission{
				Name:     fmt.Sprintf("%s:%s", p.Resource, p.Action),
				Resource: p.Resource,
				Action:   p.Action,
			}
			if err := s.db.Where("name = ?", perm.Name).FirstOrCreate(&perm).Error; err != nil {
				return err
			}

			var rp RolePermission
			result := s.db.Where("role = ? AND permission_id = ?", role, perm.ID).First(&rp)
			if result.Error != nil {
				if err := s.db.Create(&RolePermission{
					Role:         role,
					PermissionID: perm.ID,
				}).Error; err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// AssignRole 为用户分配角色，已存在则更新
func (s *MySQLRBACStore) AssignRole(userID uint, role string) error {
	var roleModel RoleModel
	if err := s.db.First(&roleModel, "name = ?", role).Error; err != nil {
		return fmt.Errorf("auth: 无效角色 %s", role)
	}

	var existing UserRole
	result := s.db.Where("user_id = ?", userID).First(&existing)
	if result.Error == nil {
		return s.db.Model(&existing).Update("role", role).Error
	}

	return s.db.Create(&UserRole{
		UserID: userID,
		Role:   role,
	}).Error
}

// GetRoles 获取用户的角色列表
func (s *MySQLRBACStore) GetRoles(userID uint) ([]string, error) {
	var roles []string
	err := s.db.Model(&UserRole{}).Where("user_id = ?", userID).Pluck("role", &roles).Error
	if err != nil || len(roles) == 0 {
		return []string{string(RoleUser)}, nil
	}
	return roles, nil
}

// GetPermissions 通过 JOIN 查询获取角色列表对应的权限
func (s *MySQLRBACStore) GetPermissions(roles []string) ([]Permission, error) {
	if len(roles) == 0 {
		return nil, nil
	}
	var perms []Permission
	err := s.db.Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("role_permissions.role IN ?", roles).
		Select("DISTINCT permissions.*").
		Find(&perms).Error
	return perms, err
}

// CreatePermission 创建权限
func (s *MySQLRBACStore) CreatePermission(name, resource, action string) (*Permission, error) {
	perm := Permission{
		Name:     name,
		Resource: resource,
		Action:   action,
	}
	if err := s.db.Create(&perm).Error; err != nil {
		return nil, fmt.Errorf("auth: 创建权限失败: %w", err)
	}
	return &perm, nil
}

// ListPermissions 列出所有权限
func (s *MySQLRBACStore) ListPermissions() ([]Permission, error) {
	var perms []Permission
	if err := s.db.Order("id ASC").Find(&perms).Error; err != nil {
		return nil, err
	}
	return perms, nil
}

// UpdatePermission 更新权限
func (s *MySQLRBACStore) UpdatePermission(id uint, name, resource, action string) error {
	result := s.db.Model(&Permission{}).Where("id = ?", id).Updates(map[string]any{
		"name":     name,
		"resource": resource,
		"action":   action,
	})
	if result.RowsAffected == 0 {
		return fmt.Errorf("auth: 权限 %d 不存在", id)
	}
	return result.Error
}

// DeletePermission 删除权限
func (s *MySQLRBACStore) DeletePermission(id uint) error {
	// Delete role-permission associations first
	s.db.Where("permission_id = ?", id).Delete(&RolePermission{})
	result := s.db.Delete(&Permission{}, id)
	if result.RowsAffected == 0 {
		return fmt.Errorf("auth: 权限 %d 不存在", id)
	}
	return result.Error
}

// GetPermissionByID 根据ID获取权限
func (s *MySQLRBACStore) GetPermissionByID(id uint) (*Permission, error) {
	var perm Permission
	if err := s.db.First(&perm, id).Error; err != nil {
		return nil, fmt.Errorf("auth: 权限 %d 不存在", id)
	}
	return &perm, nil
}

// CreateRole 创建角色
func (s *MySQLRBACStore) CreateRole(name string) error {
	return s.db.Create(&RoleModel{Name: name}).Error
}

// ListRoles 列出所有角色
func (s *MySQLRBACStore) ListRoles() ([]RoleModel, error) {
	var roles []RoleModel
	if err := s.db.Order("name ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

// UpdateRole 更新角色名称
func (s *MySQLRBACStore) UpdateRole(oldName, newName string) error {
	tx := s.db.Begin()

	var role RoleModel
	if err := tx.First(&role, "name = ?", oldName).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("auth: 角色 %s 不存在", oldName)
	}

	// Update role name in all related tables
	if err := tx.Model(&RoleModel{}).Where("name = ?", oldName).Update("name", newName).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Model(&RolePermission{}).Where("role = ?", oldName).Update("role", newName).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Model(&UserRole{}).Where("role = ?", oldName).Update("role", newName).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// DeleteRole 删除角色
func (s *MySQLRBACStore) DeleteRole(name string) error {
	if name == string(RoleAdmin) {
		return fmt.Errorf("auth: 不能删除管理员角色")
	}

	tx := s.db.Begin()

	if err := tx.Where("name = ?", name).Delete(&RoleModel{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Where("role = ?", name).Delete(&RolePermission{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Where("role = ?", name).Delete(&UserRole{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// SetRolePermissions 设置角色的权限列表（全量替换）
func (s *MySQLRBACStore) SetRolePermissions(role string, permissionIDs []uint) error {
	var roleModel RoleModel
	if err := s.db.First(&roleModel, "name = ?", role).Error; err != nil {
		return fmt.Errorf("auth: 角色 %s 不存在", role)
	}

	tx := s.db.Begin()

	// Remove all existing permissions for this role
	if err := tx.Where("role = ?", role).Delete(&RolePermission{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Add new permissions
	for _, pid := range permissionIDs {
		if err := tx.Create(&RolePermission{
			Role:         role,
			PermissionID: pid,
		}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("auth: 权限 %d 不存在", pid)
		}
	}

	return tx.Commit().Error
}

// GetRolePermissions 获取角色的权限列表
func (s *MySQLRBACStore) GetRolePermissions(role string) ([]Permission, error) {
	var perms []Permission
	err := s.db.Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("role_permissions.role = ?", role).
		Find(&perms).Error
	return perms, err
}

// GetUserPermissions 获取用户的所有权限（通过角色）
func (s *MySQLRBACStore) GetUserPermissions(userID uint) ([]Permission, error) {
	roles, err := s.GetRoles(userID)
	if err != nil {
		return nil, err
	}
	return s.GetPermissions(roles)
}

// RemoveUserRole 移除用户的某个角色
func (s *MySQLRBACStore) RemoveUserRole(userID uint, role string) error {
	result := s.db.Where("user_id = ? AND role = ?", userID, role).Delete(&UserRole{})
	if result.RowsAffected == 0 {
		return fmt.Errorf("auth: 用户 %d 没有角色 %s", userID, role)
	}
	return result.Error
}

// RBACService 基于角色的访问控制服务
type RBACService struct {
	store RBACStore
}

// NewRBACService 创建 RBAC 服务实例
func NewRBACService(store RBACStore) *RBACService {
	return &RBACService{store: store}
}

// AssignRole 为用户分配角色（委托给存储层）
func (s *RBACService) AssignRole(userID uint, role string) error {
	return s.store.AssignRole(userID, role)
}

// GetRoles 获取用户角色列表
func (s *RBACService) GetRoles(userID uint) []string {
	roles, _ := s.store.GetRoles(userID)
	return roles
}

// HasPermission 检查用户是否拥有指定资源的操作权限
func (s *RBACService) HasPermission(userID uint, resource, action string) bool {
	roles, err := s.store.GetRoles(userID)
	if err != nil {
		return false
	}

	perms, err := s.store.GetPermissions(roles)
	if err != nil {
		return false
	}

	for _, p := range perms {
		if p.Resource == "*" && p.Action == "*" {
			return true
		}
		if p.Resource == resource && (p.Action == "*" || p.Action == action) {
			return true
		}
	}
	return false
}

// RequirePermission 生成权限检查闭包，无权限时返回错误
func (s *RBACService) RequirePermission(resource, action string) func(userID uint) error {
	return func(userID uint) error {
		if !s.HasPermission(userID, resource, action) {
			return fmt.Errorf("auth: 权限不足 (%s:%s)", resource, action)
		}
		return nil
	}
}

// Permission CRUD
func (s *RBACService) CreatePermission(name, resource, action string) (*Permission, error) {
	return s.store.CreatePermission(name, resource, action)
}

func (s *RBACService) ListPermissions() ([]Permission, error) {
	return s.store.ListPermissions()
}

func (s *RBACService) UpdatePermission(id uint, name, resource, action string) error {
	return s.store.UpdatePermission(id, name, resource, action)
}

func (s *RBACService) DeletePermission(id uint) error {
	return s.store.DeletePermission(id)
}

func (s *RBACService) GetPermissionByID(id uint) (*Permission, error) {
	return s.store.GetPermissionByID(id)
}

// Role CRUD
func (s *RBACService) CreateRole(name string) error {
	return s.store.CreateRole(name)
}

func (s *RBACService) ListRoles() ([]RoleModel, error) {
	return s.store.ListRoles()
}

func (s *RBACService) UpdateRole(oldName, newName string) error {
	return s.store.UpdateRole(oldName, newName)
}

func (s *RBACService) DeleteRole(name string) error {
	return s.store.DeleteRole(name)
}

// Role-Permission binding
func (s *RBACService) SetRolePermissions(role string, permissionIDs []uint) error {
	return s.store.SetRolePermissions(role, permissionIDs)
}

func (s *RBACService) GetRolePermissions(role string) ([]Permission, error) {
	return s.store.GetRolePermissions(role)
}

// User permissions
func (s *RBACService) GetUserPermissions(userID uint) ([]Permission, error) {
	return s.store.GetUserPermissions(userID)
}

func (s *RBACService) RemoveUserRole(userID uint, role string) error {
	return s.store.RemoveUserRole(userID, role)
}

// HasRole 检查用户是否有指定角色
func (s *RBACService) HasRole(userID uint, role string) bool {
	roles := s.GetRoles(userID)
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// IsAdmin 检查用户是否为管理员
func (s *RBACService) IsAdmin(userID uint) bool {
	return s.HasPermission(userID, "*", "*") || s.HasRole(userID, string(RoleAdmin))
}
