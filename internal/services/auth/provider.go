package auth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type SocialUser struct {
	OpenID    string `json:"open_id"`
	UnionID   string `json:"union_id,omitempty"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Gender    int    `json:"gender,omitempty"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
}

type SocialProvider interface {
	Name() string
	AuthURL(state string) string
	Exchange(ctx context.Context, code string) (string, error)
	GetUserInfo(ctx context.Context, accessToken string) (*SocialUser, error)
}

type SocialAccount struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID    uint      `json:"user_id" gorm:"index"`
	Provider  string    `json:"provider" gorm:"size:50"`
	OpenID    string    `json:"open_id" gorm:"size:255"`
	UnionID   string    `json:"union_id,omitempty" gorm:"size:255"`
	Nickname  string    `json:"nickname" gorm:"size:255"`
	AvatarURL string    `json:"avatar_url,omitempty" gorm:"size:512"`
	CreatedAt time.Time `json:"created_at"`
}

type SocialStore interface {
	Bind(ctx context.Context, userID uint, provider string, info *SocialUser) error
	Unbind(ctx context.Context, userID uint, provider string) error
	FindByProvider(provider, openID string) (*SocialAccount, error)
	ListByUser(userID uint) ([]SocialAccount, error)
}

type InMemorySocialStore struct {
	mu       sync.RWMutex
	accounts []SocialAccount
	seq      uint
}

func NewInMemorySocialStore() *InMemorySocialStore {
	return &InMemorySocialStore{}
}

func (s *InMemorySocialStore) Bind(_ context.Context, userID uint, provider string, info *SocialUser) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, a := range s.accounts {
		if a.UserID == userID && a.Provider == provider {
			return fmt.Errorf("auth: 已绑定 %s 账号", provider)
		}
	}

	s.seq++
	s.accounts = append(s.accounts, SocialAccount{
		ID:        s.seq,
		UserID:    userID,
		Provider:  provider,
		OpenID:    info.OpenID,
		UnionID:   info.UnionID,
		Nickname:  info.Nickname,
		AvatarURL: info.AvatarURL,
		CreatedAt: time.Now(),
	})
	return nil
}

func (s *InMemorySocialStore) Unbind(_ context.Context, userID uint, provider string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, a := range s.accounts {
		if a.UserID == userID && a.Provider == provider {
			s.accounts = append(s.accounts[:i], s.accounts[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("auth: 未绑定 %s 账号", provider)
}

func (s *InMemorySocialStore) FindByProvider(provider, openID string) (*SocialAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, a := range s.accounts {
		if a.Provider == provider && a.OpenID == openID {
			return &a, nil
		}
	}
	return nil, fmt.Errorf("auth: 未找到 %s 账号", provider)
}

func (s *InMemorySocialStore) ListByUser(userID uint) ([]SocialAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]SocialAccount, 0)
	for _, a := range s.accounts {
		if a.UserID == userID {
			result = append(result, a)
		}
	}
	return result, nil
}
