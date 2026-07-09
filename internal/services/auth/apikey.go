package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type ApiKey struct {
	ID         uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	Name       string          `json:"name" gorm:"size:100;not null"`
	KeyPrefix  string          `json:"key_prefix" gorm:"size:10;not null"`
	KeyHash    string          `json:"-" gorm:"uniqueIndex;size:64;not null"`
	UserID     uint            `json:"user_id" gorm:"index;not null"`
	Scopes     string          `json:"scopes" gorm:"size:500;default:'*:*'"`
	LastUsedAt *time.Time      `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time      `json:"expires_at,omitempty"`
	Status     string          `json:"status" gorm:"size:20;default:active;index"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	DeletedAt  *gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

const apiKeyPrefix = "gbk_"

type ApiKeyStore interface {
	Create(key *ApiKey) error
	FindByHash(hash string) (*ApiKey, error)
	ListByUser(userID uint) ([]ApiKey, error)
	ListAll(page, pageSize int) ([]ApiKey, int64, error)
	Update(id uint, updates map[string]any) error
	Delete(id uint) error
}

type InMemoryApiKeyStore struct {
	mu   sync.RWMutex
	keys []ApiKey
	seq  uint
}

func NewInMemoryApiKeyStore() *InMemoryApiKeyStore {
	return &InMemoryApiKeyStore{}
}

func (s *InMemoryApiKeyStore) Create(key *ApiKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	key.ID = s.seq
	key.CreatedAt = time.Now()
	key.UpdatedAt = time.Now()
	s.keys = append(s.keys, *key)
	return nil
}

func (s *InMemoryApiKeyStore) FindByHash(hash string) (*ApiKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, k := range s.keys {
		if k.KeyHash == hash && k.DeletedAt == nil {
			return &k, nil
		}
	}
	return nil, fmt.Errorf("apikey: 密钥不存在")
}

func (s *InMemoryApiKeyStore) ListByUser(userID uint) ([]ApiKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []ApiKey
	for _, k := range s.keys {
		if k.UserID == userID && k.DeletedAt == nil {
			result = append(result, k)
		}
	}
	return result, nil
}

func (s *InMemoryApiKeyStore) ListAll(page, pageSize int) ([]ApiKey, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []ApiKey
	for _, k := range s.keys {
		if k.DeletedAt == nil {
			filtered = append(filtered, k)
		}
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

	result := make([]ApiKey, len(filtered[start:end]))
	copy(result, filtered[start:end])
	return result, total, nil
}

func (s *InMemoryApiKeyStore) Update(id uint, updates map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, k := range s.keys {
		if k.ID == id && k.DeletedAt == nil {
			if v, ok := updates["name"]; ok {
				s.keys[i].Name = v.(string)
			}
			if v, ok := updates["scopes"]; ok {
				s.keys[i].Scopes = v.(string)
			}
			if v, ok := updates["status"]; ok {
				s.keys[i].Status = v.(string)
			}
			if v, ok := updates["last_used_at"]; ok {
				if t, ok2 := v.(*time.Time); ok2 {
					s.keys[i].LastUsedAt = t
				}
			}
			if v, ok := updates["expires_at"]; ok {
				if t, ok2 := v.(*time.Time); ok2 {
					s.keys[i].ExpiresAt = t
				}
			}
			s.keys[i].UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("apikey: 密钥 %d 不存在", id)
}

func (s *InMemoryApiKeyStore) Delete(id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, k := range s.keys {
		if k.ID == id && k.DeletedAt == nil {
			now := gorm.DeletedAt{Time: time.Now(), Valid: true}
			s.keys[i].DeletedAt = &now
			return nil
		}
	}
	return fmt.Errorf("apikey: 密钥 %d 不存在", id)
}

type MySQLApiKeyStore struct {
	db *gorm.DB
}

func NewMySQLApiKeyStore(sqlDB *sql.DB) (*MySQLApiKeyStore, error) {
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("apikey: GORM 初始化失败: %w", err)
	}
	return &MySQLApiKeyStore{db: db}, nil
}

func (s *MySQLApiKeyStore) AutoMigrate() error {
	return s.db.AutoMigrate(&ApiKey{})
}

func (s *MySQLApiKeyStore) Create(key *ApiKey) error {
	return s.db.Create(key).Error
}

func (s *MySQLApiKeyStore) FindByHash(hash string) (*ApiKey, error) {
	var key ApiKey
	err := s.db.Where("key_hash = ? AND status = 'active'", hash).First(&key).Error
	if err != nil {
		return nil, fmt.Errorf("apikey: 密钥不存在")
	}
	return &key, nil
}

func (s *MySQLApiKeyStore) ListByUser(userID uint) ([]ApiKey, error) {
	var keys []ApiKey
	err := s.db.Where("user_id = ?", userID).Order("id DESC").Find(&keys).Error
	return keys, err
}

func (s *MySQLApiKeyStore) ListAll(page, pageSize int) ([]ApiKey, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var total int64
	if err := s.db.Model(&ApiKey{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var keys []ApiKey
	offset := (page - 1) * pageSize
	err := s.db.Offset(offset).Limit(pageSize).Order("id DESC").Find(&keys).Error
	return keys, total, err
}

func (s *MySQLApiKeyStore) Update(id uint, updates map[string]any) error {
	updates["updated_at"] = time.Now()
	return s.db.Model(&ApiKey{}).Where("id = ?", id).Updates(updates).Error
}

func (s *MySQLApiKeyStore) Delete(id uint) error {
	return s.db.Delete(&ApiKey{}, id).Error
}

type ApiKeyService struct {
	store ApiKeyStore
}

func NewApiKeyService(store ApiKeyStore) *ApiKeyService {
	return &ApiKeyService{store: store}
}

type GeneratedKey struct {
	ApiKey
	RawKey string `json:"raw_key"`
}

func (s *ApiKeyService) GenerateKey(name string, userID uint, scopes string, expiresAt *time.Time) (*GeneratedKey, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("apikey: 生成随机密钥失败: %w", err)
	}

	rawKey := apiKeyPrefix + hex.EncodeToString(raw)
	hash := s.hashKey(rawKey)
	prefix := rawKey[:10]

	key := &ApiKey{
		Name:      name,
		KeyPrefix: prefix,
		KeyHash:   hash,
		UserID:    userID,
		Scopes:    scopes,
		ExpiresAt: expiresAt,
		Status:    "active",
	}

	if err := s.store.Create(key); err != nil {
		return nil, err
	}

	return &GeneratedKey{ApiKey: *key, RawKey: rawKey}, nil
}

func (s *ApiKeyService) ValidateKey(rawKey string) (*ApiKey, error) {
	hash := s.hashKey(rawKey)
	key, err := s.store.FindByHash(hash)
	if err != nil {
		return nil, err
	}

	if key.Status != "active" {
		return nil, fmt.Errorf("apikey: 密钥已被禁用")
	}

	if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
		return nil, fmt.Errorf("apikey: 密钥已过期")
	}

	now := time.Now()
	_ = s.store.Update(key.ID, map[string]any{"last_used_at": &now})
	key.LastUsedAt = &now

	return key, nil
}

func (s *ApiKeyService) ListAll(page, pageSize int) ([]ApiKey, int64, error) {
	return s.store.ListAll(page, pageSize)
}

func (s *ApiKeyService) ListByUser(userID uint) ([]ApiKey, error) {
	return s.store.ListByUser(userID)
}

func (s *ApiKeyService) Update(id uint, updates map[string]any) error {
	return s.store.Update(id, updates)
}

func (s *ApiKeyService) Delete(id uint) error {
	return s.store.Delete(id)
}

func (s *ApiKeyService) hashKey(rawKey string) string {
	h := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(h[:])
}
