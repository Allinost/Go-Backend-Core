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

type UserStore interface {
	CreateUser(req RegisterRequest) (*User, error)
	FindByUsername(username string) (*User, error)
	FindByID(id uint) (*User, error)
	VerifyPassword(username, password string) (*User, error)
}

type InMemoryUserStore struct {
	mu    sync.RWMutex
	users []User
	seq   uint
}

func NewInMemoryUserStore() *InMemoryUserStore {
	return &InMemoryUserStore{}
}

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
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.users = append(s.users, user)
	return &s.users[len(s.users)-1], nil
}

func (s *InMemoryUserStore) FindByUsername(username string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, u := range s.users {
		if u.Username == username {
			return &u, nil
		}
	}
	return nil, fmt.Errorf("auth: 用户 %s 不存在", username)
}

func (s *InMemoryUserStore) FindByID(id uint) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, u := range s.users {
		if u.ID == id {
			return &u, nil
		}
	}
	return nil, fmt.Errorf("auth: 用户 %d 不存在", id)
}

func (s *InMemoryUserStore) VerifyPassword(username, password string) (*User, error) {
	user, err := s.FindByUsername(username)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, fmt.Errorf("auth: 密码错误")
	}

	return user, nil
}

type MySQLUserStore struct {
	db *gorm.DB
}

func NewMySQLUserStore(sqlDB *sql.DB) (*MySQLUserStore, error) {
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("auth: GORM 初始化失败: %w", err)
	}
	return &MySQLUserStore{db: db}, nil
}

func (s *MySQLUserStore) AutoMigrate() error {
	return s.db.AutoMigrate(&User{})
}

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
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, fmt.Errorf("auth: 创建用户失败: %w", err)
	}
	return &user, nil
}

func (s *MySQLUserStore) FindByUsername(username string) (*User, error) {
	var user User
	err := s.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, fmt.Errorf("auth: 用户 %s 不存在", username)
	}
	return &user, nil
}

func (s *MySQLUserStore) FindByID(id uint) (*User, error) {
	var user User
	err := s.db.First(&user, id).Error
	if err != nil {
		return nil, fmt.Errorf("auth: 用户 %d 不存在", id)
	}
	return &user, nil
}

func (s *MySQLUserStore) VerifyPassword(username, password string) (*User, error) {
	user, err := s.FindByUsername(username)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, fmt.Errorf("auth: 密码错误")
	}

	return user, nil
}
