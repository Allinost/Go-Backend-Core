package configcenter

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Store 配置持久化接口
type Store interface {
	Load(ctx context.Context) (map[string]json.RawMessage, error)
	Save(ctx context.Context, changes map[string]json.RawMessage, operator, note string) error
	Reset(ctx context.Context, keys []string, operator, note string) error
	History(ctx context.Context, key string, limit int) ([]HistoryEntry, error)
}

// HistoryEntry 配置变更历史
type HistoryEntry struct {
	Key       string          `json:"key"`
	OldValue  json.RawMessage `json:"old_value,omitempty"`
	NewValue  json.RawMessage `json:"new_value,omitempty"`
	Version   int64           `json:"version"`
	Operator  string          `json:"operator"`
	Note      string          `json:"note"`
	CreatedAt time.Time       `json:"created_at"`
}

// MemoryStore 内存存储，用于测试或数据库未就绪时降级。
type MemoryStore struct {
	mu       sync.RWMutex
	values   map[string]json.RawMessage
	versions map[string]int64
	history  []HistoryEntry
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		values:   make(map[string]json.RawMessage),
		versions: make(map[string]int64),
	}
}

func (s *MemoryStore) Load(context.Context) (map[string]json.RawMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]json.RawMessage, len(s.values))
	for k, v := range s.values {
		out[k] = append(json.RawMessage(nil), v...)
	}
	return out, nil
}

func (s *MemoryStore) Save(_ context.Context, changes map[string]json.RawMessage, operator, note string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, value := range changes {
		old := s.values[key]
		version := s.versions[key] + 1
		s.values[key] = append(json.RawMessage(nil), value...)
		s.versions[key] = version
		s.history = append(s.history, HistoryEntry{
			Key:       key,
			OldValue:  append(json.RawMessage(nil), old...),
			NewValue:  append(json.RawMessage(nil), value...),
			Version:   version,
			Operator:  operator,
			Note:      note,
			CreatedAt: time.Now().UTC(),
		})
	}
	return nil
}

func (s *MemoryStore) Reset(_ context.Context, keys []string, operator, note string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range keys {
		old := s.values[key]
		version := s.versions[key] + 1
		delete(s.values, key)
		s.versions[key] = version
		s.history = append(s.history, HistoryEntry{
			Key:       key,
			OldValue:  append(json.RawMessage(nil), old...),
			NewValue:  json.RawMessage("null"),
			Version:   version,
			Operator:  operator,
			Note:      note,
			CreatedAt: time.Now().UTC(),
		})
	}
	return nil
}

func (s *MemoryStore) History(_ context.Context, key string, limit int) ([]HistoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 50
	}
	out := make([]HistoryEntry, 0, limit)
	for i := len(s.history) - 1; i >= 0 && len(out) < limit; i-- {
		if key == "" || s.history[i].Key == key {
			out = append(out, s.history[i])
		}
	}
	return out, nil
}

// SQLStore PostgreSQL 存储。表由 migrations 创建，不在运行时代码中建表。
type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

// Init 检查配置表是否存在；表未迁移时返回错误，由调用方降级内存存储。
func (s *SQLStore) Init(ctx context.Context) error {
	if s.db == nil {
		return errors.New("configcenter: db 为空")
	}
	var one int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM system_config LIMIT 1`).Scan(&one); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("configcenter: system_config 不可用: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM system_config_history LIMIT 1`).Scan(&one); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("configcenter: system_config_history 不可用: %w", err)
	}
	return nil
}

func (s *SQLStore) Load(ctx context.Context) (map[string]json.RawMessage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value::text FROM system_config`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]json.RawMessage)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		out[key] = json.RawMessage(value)
	}
	return out, rows.Err()
}

func (s *SQLStore) Save(ctx context.Context, changes map[string]json.RawMessage, operator, note string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for key, value := range changes {
		old, version, err := loadForUpdate(ctx, tx, key)
		if err != nil {
			return err
		}
		newVersion := version + 1
		raw := string(value)
		if raw == "" {
			raw = "null"
		}
		checksum := sha256.Sum256([]byte(raw))
		_, err = tx.ExecContext(ctx, `
			INSERT INTO system_config (key, value, version, checksum, updated_by, updated_at)
			VALUES ($1, $2::jsonb, $3, $4, $5, now())
			ON CONFLICT (key) DO UPDATE SET
				value = EXCLUDED.value,
				version = EXCLUDED.version,
				checksum = EXCLUDED.checksum,
				updated_by = EXCLUDED.updated_by,
				updated_at = now()`,
			key, raw, newVersion, hex.EncodeToString(checksum[:]), operator)
		if err != nil {
			return err
		}
		if err := insertHistory(ctx, tx, key, old, value, newVersion, operator, note); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLStore) Reset(ctx context.Context, keys []string, operator, note string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, key := range keys {
		old, version, err := loadForUpdate(ctx, tx, key)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM system_config WHERE key = $1`, key); err != nil {
			return err
		}
		if err := insertHistory(ctx, tx, key, old, json.RawMessage("null"), version+1, operator, note); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLStore) History(ctx context.Context, key string, limit int) ([]HistoryEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `SELECT key, COALESCE(old_value::text,''), new_value::text, version, operator, note, created_at
		FROM system_config_history`
	args := []any{}
	if key != "" {
		query += ` WHERE key = $1`
		args = append(args, key)
	}
	query += ` ORDER BY id DESC LIMIT ` + fmt.Sprint(limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HistoryEntry
	for rows.Next() {
		var e HistoryEntry
		var oldValue, newValue string
		if err := rows.Scan(&e.Key, &oldValue, &newValue, &e.Version, &e.Operator, &e.Note, &e.CreatedAt); err != nil {
			return nil, err
		}
		if oldValue != "" {
			e.OldValue = json.RawMessage(oldValue)
		}
		e.NewValue = json.RawMessage(newValue)
		out = append(out, e)
	}
	return out, rows.Err()
}

func loadForUpdate(ctx context.Context, tx *sql.Tx, key string) (json.RawMessage, int64, error) {
	var value string
	var version int64
	err := tx.QueryRowContext(ctx, `SELECT value::text, version FROM system_config WHERE key = $1 FOR UPDATE`, key).Scan(&value, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	return json.RawMessage(value), version, nil
}

func insertHistory(ctx context.Context, tx *sql.Tx, key string, oldValue, newValue json.RawMessage, version int64, operator, note string) error {
	oldRaw := "null"
	if len(oldValue) > 0 {
		oldRaw = string(oldValue)
	}
	newRaw := "null"
	if len(newValue) > 0 {
		newRaw = string(newValue)
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO system_config_history (key, old_value, new_value, version, operator, note, created_at)
		VALUES ($1, $2::jsonb, $3::jsonb, $4, $5, $6, now())`,
		key, oldRaw, newRaw, version, operator, note)
	return err
}
