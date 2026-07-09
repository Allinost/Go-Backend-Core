package auth

import (
	"time"

	"gorm.io/gorm"
)

type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusInactive  UserStatus = "inactive"
	UserStatusSuspended UserStatus = "suspended"
)

type User struct {
	ID        uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	Username  string          `json:"username" gorm:"uniqueIndex;size:100;not null"`
	Password  string          `json:"-" gorm:"size:255;not null"`
	Nickname  string          `json:"nickname,omitempty" gorm:"size:100"`
	Email     string          `json:"email,omitempty" gorm:"size:255"`
	AvatarURL string          `json:"avatar_url,omitempty" gorm:"size:512"`
	Phone     string          `json:"phone,omitempty" gorm:"size:50"`
	Status    UserStatus      `json:"status" gorm:"size:20;default:active;index"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	DeletedAt *gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6,max=128"`
	Nickname string `json:"nickname,omitempty"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UpdateUserRequest struct {
	Nickname  *string `json:"nickname,omitempty"`
	Email     *string `json:"email,omitempty"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	Phone     *string `json:"phone,omitempty"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=128"`
}

type TokenResponse struct {
	Token     string `json:"token"`
	ExpiresIn int64  `json:"expires_in"`
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
}
