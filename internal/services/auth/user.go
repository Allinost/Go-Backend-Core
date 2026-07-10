package auth

import (
	"time"

	"gorm.io/gorm"
)

// UserStatus 用户状态类型
type UserStatus string

const (
	UserStatusActive    UserStatus = "active"    // 激活
	UserStatusInactive  UserStatus = "inactive"  // 未激活
	UserStatusSuspended UserStatus = "suspended" // 已停用
)

// User 系统用户模型
type User struct {
	ID        uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	Username  string          `json:"username" gorm:"uniqueIndex;size:100;not null"`
	Password  string          `json:"-" gorm:"size:255;not null"` // bcrypt 加密存储，JSON 序列化时隐藏
	Nickname  string          `json:"nickname,omitempty" gorm:"size:100"`
	Email     string          `json:"email,omitempty" gorm:"size:255"`
	AvatarURL string          `json:"avatar_url,omitempty" gorm:"size:512"`
	Phone     string          `json:"phone,omitempty" gorm:"size:50"`
	Status    UserStatus      `json:"status" gorm:"size:20;default:active;index"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	DeletedAt *gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// RegisterRequest 用户注册请求参数
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6,max=128"`
	Nickname string `json:"nickname,omitempty"`
}

// LoginRequest 用户登录请求参数
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// UpdateUserRequest 用户信息更新请求参数（指针字段表示可选更新）
type UpdateUserRequest struct {
	Nickname  *string `json:"nickname,omitempty"`
	Email     *string `json:"email,omitempty"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	Phone     *string `json:"phone,omitempty"`
}

// ChangePasswordRequest 修改密码请求参数
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=128"`
}

// TokenResponse 登录/刷新 token 的响应结构
type TokenResponse struct {
	Token     string `json:"token"`
	ExpiresIn int64  `json:"expires_in"`
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
}
