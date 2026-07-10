package auth

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// UserStore 用户数据存储接口
type UserStore interface {
	CreateUser(req RegisterRequest) (*User, error)
	FindByUsername(username string) (*User, error)
	FindByID(id uint) (*User, error)
	VerifyPassword(username, password string) (*User, error)
	ListUsers(page, pageSize int, search string) ([]User, int64, error)
	UpdateUser(id uint, req UpdateUserRequest) (*User, error)
	DeleteUser(id uint) error
	ChangePassword(id uint, oldPassword, newPassword string) error
}

// InMemoryUserStore 基于内存的用户数据存储实现
type InMemoryUserStore struct {
	mu    sync.RWMutex
	users []User
	seq   uint
}

// NewInMemoryUserStore 创建内存用户存储实例
func NewInMemoryUserStore() *InMemoryUserStore {
	return &InMemoryUserStore{}
}

// CreateUser 创建新用户，检查用户名唯一性并对密码进行 bcrypt 加密
func (s *InMemoryUserStore) CreateUser(req RegisterRequest) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, u := range s.users {
		if u.Username == req.Username {
			return nil, fmt.Errorf("auth: 用户名 %s 已存在", req.Username)
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("auth: 密码加密失败: %w", err)
	}

	s.seq++
	now := time.Now()
	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Username
	}

	user := User{
		ID:        s.seq,
		Username:  req.Username,
		Password:  string(hash),
		Nickname:  nickname,
		Status:    UserStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.users = append(s.users, user)
	return &s.users[len(s.users)-1], nil
}

// FindByUsername 根据用户名查找未删除的用户
func (s *InMemoryUserStore) FindByUsername(username string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, u := range s.users {
		if u.Username == username && u.DeletedAt == nil {
			return &u, nil
		}
	}
	return nil, fmt.Errorf("auth: 用户 %s 不存在", username)
}

// FindByID 根据用户 ID 查找未删除的用户
func (s *InMemoryUserStore) FindByID(id uint) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, u := range s.users {
		if u.ID == id && u.DeletedAt == nil {
			return &u, nil
		}
	}
	return nil, fmt.Errorf("auth: 用户 %d 不存在", id)
}

// VerifyPassword 验证用户名密码，同时检查用户状态是否为激活
func (s *InMemoryUserStore) VerifyPassword(username, password string) (*User, error) {
	user, err := s.FindByUsername(username)
	if err != nil {
		return nil, err
	}

	if user.Status != UserStatusActive {
		return nil, fmt.Errorf("auth: 用户已被禁用")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, fmt.Errorf("auth: 密码错误")
	}

	return user, nil
}

// ListUsers 分页查询用户列表，支持按用户名/昵称精确搜索
func (s *InMemoryUserStore) ListUsers(page, pageSize int, search string) ([]User, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []User
	for _, u := range s.users {
		if u.DeletedAt != nil {
			continue
		}
		if search != "" {
			if u.Username == search || u.Nickname == search {
				filtered = append(filtered, u)
			}
			continue
		}
		filtered = append(filtered, u)
	}

	total := int64(len(filtered))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	if start >= len(filtered) {
		return nil, total, nil
	}

	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	result := make([]User, len(filtered[start:end]))
	copy(result, filtered[start:end])
	return result, total, nil
}

// UpdateUser 更新用户信息（昵称、邮箱、头像、电话）
func (s *InMemoryUserStore) UpdateUser(id uint, req UpdateUserRequest) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, u := range s.users {
		if u.ID == id && u.DeletedAt == nil {
			if req.Nickname != nil {
				s.users[i].Nickname = *req.Nickname
			}
			if req.Email != nil {
				s.users[i].Email = *req.Email
			}
			if req.AvatarURL != nil {
				s.users[i].AvatarURL = *req.AvatarURL
			}
			if req.Phone != nil {
				s.users[i].Phone = *req.Phone
			}
			s.users[i].UpdatedAt = time.Now()
			return &s.users[i], nil
		}
	}
	return nil, fmt.Errorf("auth: 用户 %d 不存在", id)
}

// DeleteUser 软删除用户（设置 DeletedAt 时间戳）
func (s *InMemoryUserStore) DeleteUser(id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, u := range s.users {
		if u.ID == id && u.DeletedAt == nil {
			now := gorm.DeletedAt{Time: time.Now(), Valid: true}
			s.users[i].DeletedAt = &now
			return nil
		}
	}
	return fmt.Errorf("auth: 用户 %d 不存在", id)
}

// ChangePassword 修改用户密码，需验证原密码
func (s *InMemoryUserStore) ChangePassword(id uint, oldPassword, newPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, u := range s.users {
		if u.ID == id && u.DeletedAt == nil {
			if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(oldPassword)); err != nil {
				return fmt.Errorf("auth: 原密码错误")
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
			if err != nil {
				return fmt.Errorf("auth: 密码加密失败: %w", err)
			}
			s.users[i].Password = string(hash)
			s.users[i].UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("auth: 用户 %d 不存在", id)
}

// MySQLUserStore 基于 MySQL 的用户数据存储实现
type MySQLUserStore struct {
	db *gorm.DB
}

// NewMySQLUserStore 使用现有 sql.DB 创建 MySQL 用户存储
func NewMySQLUserStore(sqlDB *sql.DB) (*MySQLUserStore, error) {
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("auth: GORM 初始化失败: %w", err)
	}
	return &MySQLUserStore{db: db}, nil
}

// AutoMigrate 自动迁移 User 数据表
func (s *MySQLUserStore) AutoMigrate() error {
	return s.db.AutoMigrate(&User{})
}

// CreateUser 创建用户，密码自动 bcrypt 加密后存入数据库
func (s *MySQLUserStore) CreateUser(req RegisterRequest) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("auth: 密码加密失败: %w", err)
	}

	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Username
	}

	user := User{
		Username: req.Username,
		Password: string(hash),
		Nickname: nickname,
		Status:   UserStatusActive,
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, fmt.Errorf("auth: 创建用户失败: %w", err)
	}
	return &user, nil
}

// FindByUsername 根据用户名查询用户
func (s *MySQLUserStore) FindByUsername(username string) (*User, error) {
	var user User
	err := s.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, fmt.Errorf("auth: 用户 %s 不存在", username)
	}
	return &user, nil
}

// FindByID 根据用户 ID 查询用户
func (s *MySQLUserStore) FindByID(id uint) (*User, error) {
	var user User
	err := s.db.First(&user, id).Error
	if err != nil {
		return nil, fmt.Errorf("auth: 用户 %d 不存在", id)
	}
	return &user, nil
}

// VerifyPassword 验证用户名密码及用户状态
func (s *MySQLUserStore) VerifyPassword(username, password string) (*User, error) {
	user, err := s.FindByUsername(username)
	if err != nil {
		return nil, err
	}

	if user.Status != UserStatusActive {
		return nil, fmt.Errorf("auth: 用户已被禁用")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, fmt.Errorf("auth: 密码错误")
	}

	return user, nil
}

// ListUsers 分页查询用户列表，支持模糊搜索用户名和昵称
func (s *MySQLUserStore) ListUsers(page, pageSize int, search string) ([]User, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var total int64
	query := s.db.Model(&User{})
	if search != "" {
		query = query.Where("username LIKE ? OR nickname LIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []User
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// UpdateUser 更新用户信息，仅更新非 nil 字段
func (s *MySQLUserStore) UpdateUser(id uint, req UpdateUserRequest) (*User, error) {
	updates := map[string]any{}
	if req.Nickname != nil {
		updates["nickname"] = *req.Nickname
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.AvatarURL != nil {
		updates["avatar_url"] = *req.AvatarURL
	}
	if req.Phone != nil {
		updates["phone"] = *req.Phone
	}
	updates["updated_at"] = time.Now()

	if err := s.db.Model(&User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("auth: 更新用户失败: %w", err)
	}
	return s.FindByID(id)
}

// DeleteUser 硬删除用户记录
func (s *MySQLUserStore) DeleteUser(id uint) error {
	return s.db.Delete(&User{}, id).Error
}

// ChangePassword 修改密码，需验证原密码正确性
func (s *MySQLUserStore) ChangePassword(id uint, oldPassword, newPassword string) error {
	user, err := s.FindByID(id)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
		return fmt.Errorf("auth: 原密码错误")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("auth: 密码加密失败: %w", err)
	}

	return s.db.Model(&User{}).Where("id = ?", id).Update("password", string(hash)).Error
}
