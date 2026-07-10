package auth

import (
	"database/sql"
	"fmt"
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
}

// InMemoryRBACStore 基于内存的 RBAC 存储实现
type InMemoryRBACStore struct {
	permissions map[string][]Permission
	userRoles   map[uint][]string
}

// NewInMemoryRBACStore 创建内存 RBAC 存储实例
func NewInMemoryRBACStore() *InMemoryRBACStore {
	s := &InMemoryRBACStore{
		permissions: make(map[string][]Permission),
		userRoles:   make(map[uint][]string),
	}
	return s
}

// EnsureDefaultPermissions 初始化默认角色权限
func (s *InMemoryRBACStore) EnsureDefaultPermissions() error {
	for role, perms := range defaultPermissions {
		for _, p := range perms {
			perm := Permission{
				Name:     fmt.Sprintf("%s:%s", p.Resource, p.Action),
				Resource: p.Resource,
				Action:   p.Action,
			}
			s.permissions[role] = append(s.permissions[role], perm)
		}
	}
	return nil
}

// AssignRole 为用户分配角色
func (s *InMemoryRBACStore) AssignRole(userID uint, role string) error {
	if role != string(RoleAdmin) && role != string(RoleUser) {
		return fmt.Errorf("auth: 无效角色 %s", role)
	}
	s.userRoles[userID] = []string{role}
	return nil
}

// GetRoles 获取用户的角色列表
func (s *InMemoryRBACStore) GetRoles(userID uint) ([]string, error) {
	roles := s.userRoles[userID]
	if len(roles) == 0 {
		return []string{string(RoleUser)}, nil
	}
	return roles, nil
}

// GetPermissions 获取指定角色列表的权限集合（已去重）
func (s *InMemoryRBACStore) GetPermissions(roles []string) ([]Permission, error) {
	var result []Permission
	seen := make(map[string]bool)
	for _, role := range roles {
		for _, p := range s.permissions[role] {
			if !seen[p.Name] {
				result = append(result, p)
				seen[p.Name] = true
			}
		}
	}
	return result, nil
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
	return s.db.AutoMigrate(&Permission{}, &RolePermission{}, &UserRole{})
}

// EnsureDefaultPermissions 初始化默认角色权限（MySQL 实现）
func (s *MySQLRBACStore) EnsureDefaultPermissions() error {
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
	if role != string(RoleAdmin) && role != string(RoleUser) {
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
	var perms []Permission
	err := s.db.Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("role_permissions.role IN ?", roles).
		Select("DISTINCT permissions.*").
		Find(&perms).Error
	return perms, err
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
