package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/crypto"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/services/connector"
	"github.com/google/uuid"
)

const tableConnectors = "zzz_integration_connectors"

// ConnectorConfig 连接器在数据库中的配置结构，secret 为加密后的值。
type ConnectorConfig struct {
	URL            string            `json:"url"`
	Secret         string            `json:"secret,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Topics         []string          `json:"topics,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
}

// ConnectorDTO 对外返回的连接器信息，不包含 secret 明文或密文。
type ConnectorDTO struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	URL            string            `json:"url"`
	Headers        map[string]string `json:"headers,omitempty"`
	Topics         []string          `json:"topics,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	Enabled        bool              `json:"enabled"`
	SecretSet      bool              `json:"secret_set"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// ConnectorReq 创建/更新连接器请求。更新时 Secret 为 nil 表示保持不变，空字符串表示清除。
type ConnectorReq struct {
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	URL            string            `json:"url"`
	Secret         *string           `json:"secret"`
	Headers        map[string]string `json:"headers"`
	Topics         []string          `json:"topics"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	Enabled        *bool             `json:"enabled"`
}

// ConnectorManager 负责连接器配置持久化、密钥加密与注册表动态刷新。
type ConnectorManager struct {
	db       *sql.DB
	cipher   *crypto.Cipher
	registry *connector.Registry
	mu       sync.Mutex
}

// NewConnectorManager 创建连接器管理器。cipher 为空时允许读写无密钥连接器。
func NewConnectorManager(db *sql.DB, cipher *crypto.Cipher, registry *connector.Registry) *ConnectorManager {
	return &ConnectorManager{db: db, cipher: cipher, registry: registry}
}

// Load 从数据库加载启用的连接器。
func (m *ConnectorManager) Load(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range m.registry.All() {
		m.registry.Remove(item.ID())
	}
	records, err := m.listRecords(ctx, true)
	if err != nil {
		return err
	}
	for _, record := range records {
		item, buildErr := m.build(record)
		if buildErr != nil {
			logger.Warn().Err(buildErr).Str("connector", record.ID).Msg("integration: 跳过无效连接器配置")
			continue
		}
		if err := m.registry.Register(item); err != nil {
			logger.Warn().Err(err).Str("connector", record.ID).Msg("integration: 注册连接器失败")
		}
	}
	return nil
}

// List 返回全部连接器配置。
func (m *ConnectorManager) List(ctx context.Context) ([]ConnectorDTO, error) {
	records, err := m.listRecords(ctx, false)
	if err != nil {
		return nil, err
	}
	out := make([]ConnectorDTO, 0, len(records))
	for _, record := range records {
		var cfg ConnectorConfig
		if err := json.Unmarshal(record.Config, &cfg); err != nil {
			continue
		}
		out = append(out, toConnectorDTO(record, cfg))
	}
	return out, nil
}

// Create 创建连接器并刷新注册表。
func (m *ConnectorManager) Create(ctx context.Context, req *ConnectorReq) (*ConnectorDTO, error) {
	cfg, err := m.buildConfig(req, ConnectorConfig{})
	if err != nil {
		return nil, err
	}
	if _, err := m.buildWebhook(uuid.NewString(), cfg); err != nil {
		return nil, err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	id := uuid.NewString()
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	_, err = m.db.ExecContext(ctx, `INSERT INTO `+tableConnectors+`
		(id, name, type, config, enabled) VALUES (?, ?, ?, ?, ?)`,
		id, connectorName(req.Name), connectorType(req.Type), string(data), enabled)
	if err != nil {
		return nil, err
	}
	if err := m.Load(ctx); err != nil {
		return nil, err
	}
	return m.Get(ctx, id)
}

// Update 更新连接器并刷新注册表。
func (m *ConnectorManager) Update(ctx context.Context, id string, req *ConnectorReq) (*ConnectorDTO, error) {
	record, err := m.getRecord(ctx, id)
	if err != nil {
		return nil, err
	}
	var existing ConnectorConfig
	if err := json.Unmarshal(record.Config, &existing); err != nil {
		return nil, fmt.Errorf("integration: 连接器配置损坏: %w", err)
	}
	cfg, err := m.buildConfig(req, existing)
	if err != nil {
		return nil, err
	}
	if _, err := m.buildWebhook(id, cfg); err != nil {
		return nil, err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	enabled := record.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	result, err := m.db.ExecContext(ctx, `UPDATE `+tableConnectors+`
		SET name = ?, type = ?, config = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		connectorName(req.Name), connectorType(req.Type), string(data), enabled, id)
	if err != nil {
		return nil, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil, connector.ErrNotFound
	}
	if err := m.Load(ctx); err != nil {
		return nil, err
	}
	return m.Get(ctx, id)
}

// Delete 删除连接器并刷新注册表。
func (m *ConnectorManager) Delete(ctx context.Context, id string) error {
	result, err := m.db.ExecContext(ctx, `DELETE FROM `+tableConnectors+` WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return connector.ErrNotFound
	}
	return m.Load(ctx)
}

// Get 返回单个连接器。
func (m *ConnectorManager) Get(ctx context.Context, id string) (*ConnectorDTO, error) {
	record, err := m.getRecord(ctx, id)
	if err != nil {
		return nil, err
	}
	var cfg ConnectorConfig
	if err := json.Unmarshal(record.Config, &cfg); err != nil {
		return nil, err
	}
	dto := toConnectorDTO(record, cfg)
	return &dto, nil
}

func (m *ConnectorManager) buildConfig(req *ConnectorReq, existing ConnectorConfig) (ConnectorConfig, error) {
	url := strings.TrimSpace(req.URL)
	if url == "" {
		url = existing.URL
	}
	if url == "" {
		return ConnectorConfig{}, fmt.Errorf("%w: url 不能为空", connector.ErrInvalidConfig)
	}
	if req.Type != "" && !strings.EqualFold(strings.TrimSpace(req.Type), "webhook") {
		return ConnectorConfig{}, fmt.Errorf("%w: 暂只支持 webhook", connector.ErrInvalidConfig)
	}

	cfg := ConnectorConfig{
		URL:            url,
		Secret:         existing.Secret,
		Headers:        existing.Headers,
		Topics:         existing.Topics,
		TimeoutSeconds: existing.TimeoutSeconds,
	}
	if req.Headers != nil {
		cfg.Headers = cloneStringMap(req.Headers)
	}
	if req.Topics != nil {
		topics, err := normalizeTopics(req.Topics)
		if err != nil {
			return ConnectorConfig{}, err
		}
		cfg.Topics = topics
	}
	if req.TimeoutSeconds > 0 {
		if req.TimeoutSeconds > 120 {
			return ConnectorConfig{}, fmt.Errorf("%w: timeout_seconds 不能超过 120", connector.ErrInvalidConfig)
		}
		cfg.TimeoutSeconds = req.TimeoutSeconds
	}
	if req.Secret != nil {
		secret := strings.TrimSpace(*req.Secret)
		if secret == "" {
			cfg.Secret = ""
		} else {
			if m.cipher == nil {
				return ConnectorConfig{}, fmt.Errorf("%w: 未配置 crypto.master_key_hex，无法保存 secret", connector.ErrInvalidConfig)
			}
			encrypted, err := m.cipher.Encrypt(secret)
			if err != nil {
				return ConnectorConfig{}, err
			}
			cfg.Secret = encrypted
		}
	}
	return cfg, nil
}

func (m *ConnectorManager) build(record connectorRecord) (connector.Connector, error) {
	var cfg ConnectorConfig
	if err := json.Unmarshal(record.Config, &cfg); err != nil {
		return nil, fmt.Errorf("integration: 解析连接器配置失败: %w", err)
	}
	return m.buildWebhook(record.ID, cfg)
}

func (m *ConnectorManager) buildWebhook(id string, cfg ConnectorConfig) (connector.Connector, error) {
	secret := cfg.Secret
	if secret != "" {
		if m.cipher != nil {
			plain, err := m.cipher.Decrypt(secret)
			if err != nil {
				return nil, err
			}
			secret = plain
		} else if strings.HasPrefix(secret, "v1:") {
			return nil, fmt.Errorf("%w: 连接器 secret 已加密但未配置 crypto.master_key_hex", connector.ErrInvalidConfig)
		}
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	return connector.NewWebhook(connector.WebhookConfig{
		ID:      id,
		URL:     cfg.URL,
		Secret:  secret,
		Headers: cfg.Headers,
		Topics:  cfg.Topics,
		Timeout: timeout,
	})
}

func (m *ConnectorManager) listRecords(ctx context.Context, onlyEnabled bool) ([]connectorRecord, error) {
	query := `SELECT id, name, type, config, enabled, created_at, updated_at FROM ` + tableConnectors
	if onlyEnabled {
		query += " WHERE enabled = 1"
	}
	query += " ORDER BY created_at DESC"
	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]connectorRecord, 0)
	for rows.Next() {
		record, err := scanConnectorRecord(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, rows.Err()
}

func (m *ConnectorManager) getRecord(ctx context.Context, id string) (connectorRecord, error) {
	row := m.db.QueryRowContext(ctx, `SELECT id, name, type, config, enabled, created_at, updated_at
		FROM `+tableConnectors+` WHERE id = ?`, id)
	record, err := scanConnectorRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return connectorRecord{}, connector.ErrNotFound
	}
	return record, err
}

type connectorRecord struct {
	ID        string
	Name      string
	Type      string
	Config    []byte
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func scanConnectorRecord(scanner rowScanner) (connectorRecord, error) {
	var (
		record  connectorRecord
		enabled int
	)
	if err := scanner.Scan(&record.ID, &record.Name, &record.Type, &record.Config, &enabled, &record.CreatedAt, &record.UpdatedAt); err != nil {
		return connectorRecord{}, err
	}
	record.Enabled = enabled == 1
	return record, nil
}

func toConnectorDTO(record connectorRecord, cfg ConnectorConfig) ConnectorDTO {
	return ConnectorDTO{
		ID:             record.ID,
		Name:           record.Name,
		Type:           record.Type,
		URL:            cfg.URL,
		Headers:        cfg.Headers,
		Topics:         cfg.Topics,
		TimeoutSeconds: cfg.TimeoutSeconds,
		Enabled:        record.Enabled,
		SecretSet:      cfg.Secret != "",
		CreatedAt:      record.CreatedAt,
		UpdatedAt:      record.UpdatedAt,
	}
}

func connectorName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "webhook"
	}
	if len(name) > 100 {
		return name[:100]
	}
	return name
}

func connectorType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "webhook"
	}
	return value
}

func normalizeTopics(items []string) ([]string, error) {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if len(item) > 100 {
			return nil, fmt.Errorf("%w: topic 规则不能超过 100 字符", connector.ErrInvalidConfig)
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	if len(out) > 50 {
		return nil, fmt.Errorf("%w: topics 不能超过 50 条", connector.ErrInvalidConfig)
	}
	if out == nil {
		return []string{}, nil
	}
	return out, nil
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for key, value := range src {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

type rowScanner interface {
	Scan(dest ...any) error
}
