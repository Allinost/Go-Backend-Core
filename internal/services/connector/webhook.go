package connector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebhookConfig Webhook 连接器配置
type WebhookConfig struct {
	ID      string
	URL     string
	Secret  string
	Headers map[string]string
	Timeout time.Duration
}

// Webhook 通过签名 JSON POST 投递事件的连接器
type Webhook struct {
	id      string
	url     string
	secret  string
	headers map[string]string
	client  *http.Client
	now     func() time.Time
}

// NewWebhook 创建 Webhook 连接器
func NewWebhook(cfg WebhookConfig) (*Webhook, error) {
	cfg.ID = strings.TrimSpace(cfg.ID)
	cfg.URL = strings.TrimSpace(cfg.URL)
	if cfg.ID == "" || cfg.URL == "" {
		return nil, fmt.Errorf("%w: id 与 url 不能为空", ErrInvalidConfig)
	}
	parsed, err := url.Parse(cfg.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("%w: 非法 url %q", ErrInvalidConfig, cfg.URL)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Webhook{
		id:      cfg.ID,
		url:     cfg.URL,
		secret:  cfg.Secret,
		headers: cfg.Headers,
		client:  &http.Client{Timeout: timeout},
		now:     time.Now,
	}, nil
}

// ID 连接器 ID
func (w *Webhook) ID() string { return w.id }

// Type 连接器类型
func (w *Webhook) Type() string { return "webhook" }

// Send 发送签名 JSON POST
func (w *Webhook) Send(ctx context.Context, msg Message) error {
	occurredAt := msg.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = w.now().UTC()
	}
	envelope := struct {
		EventID    string            `json:"event_id"`
		Event      string            `json:"event"`
		OccurredAt time.Time         `json:"occurred_at"`
		Payload    json.RawMessage   `json:"payload"`
		Metadata   map[string]string `json:"metadata,omitempty"`
	}{
		EventID:    msg.EventID,
		Event:      msg.Event,
		OccurredAt: occurredAt,
		Payload:    msg.Payload,
		Metadata:   msg.Metadata,
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("connector/webhook: 序列化消息失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("connector/webhook: 创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "go-backend-core/integration")
	req.Header.Set("X-Integration-Event", msg.Event)
	if msg.EventID != "" {
		req.Header.Set("X-Integration-Event-ID", msg.EventID)
	}
	if w.secret != "" {
		timestamp := w.now().Unix()
		req.Header.Set("X-Integration-Timestamp", fmt.Sprintf("%d", timestamp))
		req.Header.Set("X-Integration-Signature", Sign(w.secret, timestamp, body))
	}
	for key, value := range w.headers {
		req.Header.Set(key, value)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("connector/webhook: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("connector/webhook: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(detail)))
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return nil
}
