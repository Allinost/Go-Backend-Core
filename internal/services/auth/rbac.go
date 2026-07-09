package auth

import (
	"database/sql"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

type Permission struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string    `json:"name" gorm:"uniqueIndex;size:255"`
	Resource  string    `json:"resource" gorm:"size:100"`
	Action    string    `json:"action" gorm:"size:50"`
	CreatedAt time.Time `json:"created_at"`
}

type RolePermission struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Role         string    `json:"role" gorm:"size:50;index;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	PermissionID uint      `json:"permission_id"`
	CreatedAt    time.Time `json:"created_at"`
}

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

type RBACStore interface {
	EnsureDefaultPermissions() error
	AssignRole(userID uint, role string) error
	GetRoles(userID uint) ([]string, error)
	GetPermissions(roles []string) ([]Permission, error)
}

type InMemoryRBACStore struct {
	permissions map[string][]Permission
	userRoles   map[uint][]string
}

func NewInMemoryRBACStore() *InMemoryRBACStore {
	s := &InMemoryRBACStore{
		permissions: make(map[string][]Permission),
		userRoles:   make(map[uint][]string),
	}
	return s
}

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

func (s *InMemoryRBACStore) AssignRole(userID uint, role string) error {
	if role != string(RoleAdmin) && role != string(RoleUser) {
		return fmt.Errorf("auth: 无效角色 %s", role)
	}
	s.userRoles[userID] = []string{role}
	return nil
}

func (s *InMemoryRBACStore) GetRoles(userID uint) ([]string, error) {
	roles := s.userRoles[userID]
	if len(roles) == 0 {
		return []string{string(RoleUser)}, nil
	}
	return roles, nil
}

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

type MySQLRBACStore struct {
	db *gorm.DB
}

func NewMySQLRBACStore(sqlDB *sql.DB) (*MySQLRBACStore, error) {
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("auth: RBAC GORM 初始化失败: %w", err)
	}
	return &MySQLRBACStore{db: db}, nil
}

func (s *MySQLRBACStore) AutoMigrate() error {
	return s.db.AutoMigrate(&Permission{}, &RolePermission{}, &UserRole{})
}

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

func (s *MySQLRBACStore) GetRoles(userID uint) ([]string, error) {
	var roles []string
	err := s.db.Model(&UserRole{}).Where("user_id = ?", userID).Pluck("role", &roles).Error
	if err != nil || len(roles) == 0 {
		return []string{string(RoleUser)}, nil
	}
	return roles, nil
}

func (s *MySQLRBACStore) GetPermissions(roles []string) ([]Permission, error) {
	var perms []Permission
	err := s.db.Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("role_permissions.role IN ?", roles).
		Select("DISTINCT permissions.*").
		Find(&perms).Error
	return perms, err
}

type RBACService struct {
	store RBACStore
}

func NewRBACService(store RBACStore) *RBACService {
	return &RBACService{store: store}
}

func (s *RBACService) AssignRole(userID uint, role string) error {
	return s.store.AssignRole(userID, role)
}

func (s *RBACService) GetRoles(userID uint) []string {
	roles, _ := s.store.GetRoles(userID)
	return roles
}

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

func (s *RBACService) RequirePermission(resource, action string) func(userID uint) error {
	return func(userID uint) error {
		if !s.HasPermission(userID, resource, action) {
			return fmt.Errorf("auth: 权限不足 (%s:%s)", resource, action)
		}
		return nil
	}
}
