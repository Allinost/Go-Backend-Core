package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"
)

// Role 对话角色类型
type Role string

const (
	RoleSystem    Role = "system"    // 系统角色
	RoleUser      Role = "user"      // 用户角色
	RoleAssistant Role = "assistant" // 助手角色
)

// Message 对话消息结构
type Message struct {
	Role    Role   `json:"role"`    // 消息角色
	Content string `json:"content"` // 消息内容
}

// ChatRequest 聊天请求参数
type ChatRequest struct {
	Model    string         `json:"model"`             // 模型名称
	Messages []Message      `json:"messages"`          // 消息列表
	Stream   bool           `json:"stream,omitempty"`  // 是否流式输出
	Options  map[string]any `json:"options,omitempty"` // 额外选项参数
}

// ChatResponse 聊天响应结果
type ChatResponse struct {
	Message Message `json:"message"`         // 响应消息
	Model   string  `json:"model"`           // 使用的模型
	Usage   Usage   `json:"usage,omitempty"` // token 用量统计
}

// ChatStreamChunk 流式聊天响应块
type ChatStreamChunk struct {
	Content string `json:"content"`         // 块内容
	Done    bool   `json:"done"`            // 是否结束
	Usage   Usage  `json:"usage,omitempty"` // token 用量统计
}

// EmbedRequest 嵌入向量请求参数
type EmbedRequest struct {
	Model  string   `json:"model"`  // 模型名称
	Inputs []string `json:"inputs"` // 输入文本列表
}

// EmbedResponse 嵌入向量响应结果
type EmbedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`      // 嵌入向量数组
	Model      string      `json:"model"`           // 使用的模型
	Usage      Usage       `json:"usage,omitempty"` // token 用量统计
}

// Usage token 用量统计
type Usage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`     // 提示 token 数
	CompletionTokens int `json:"completion_tokens,omitempty"` // 生成 token 数
	TotalTokens      int `json:"total_tokens,omitempty"`      // 总 token 数
}

// ModelInfo 模型信息
type ModelInfo struct {
	Name     string `json:"name"`     // 模型名称
	Provider string `json:"provider"` // 所属提供商
	Type     string `json:"type"`     // 模型类型：chat / embedding
}

// Provider AI 提供者接口
type Provider interface {
	Name() string                                                                    // 提供者名称
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)                // 非流式对话
	ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatStreamChunk, error) // 流式对话
	Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error)             // 获取嵌入向量
	Models(ctx context.Context) ([]ModelInfo, error)                                 // 获取可用模型列表
	Close() error                                                                    // 关闭提供者
}

var (
	ErrProviderNotFound   = errors.New("ai: provider not found")                  // 提供者未找到
	ErrModelNotFound      = errors.New("ai: model not found")                     // 模型未找到
	ErrStreamNotSupported = errors.New("ai: provider does not support streaming") // 提供者不支持流式
)

// RouterStrategy 路由策略类型
type RouterStrategy int

const (
	StrategyDirect   RouterStrategy = iota // 直接路由
	StrategyFallback                       // 回退路由
	StrategyWeighted                       // 加权路由
)

// RouteRule 路由规则
type RouteRule struct {
	Model    string // 模型名称
	Provider string // 主提供者
	Fallback string // 回退提供者
	Weight   int    // 权重
}

// Manager AI 提供者管理器
type Manager struct {
	mu        sync.RWMutex
	providers map[string]Provider // 已注册的提供者
	rules     []RouteRule         // 路由规则列表
	strategy  RouterStrategy      // 路由策略

	retryCount   int           // 重试次数
	retryWait    time.Duration // 初始重试等待时间
	retryMaxWait time.Duration // 最大重试等待时间
}

// NewManager 创建 AI 管理器
func NewManager() *Manager {
	return &Manager{
		providers:    make(map[string]Provider),
		strategy:     StrategyDirect,
		retryCount:   2,
		retryWait:    100 * time.Millisecond,
		retryMaxWait: 2 * time.Second,
	}
}

// RegisterProvider 注册 AI 提供者
func (m *Manager) RegisterProvider(p Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[p.Name()] = p
}

// GetProvider 获取已注册的提供者
func (m *Manager) GetProvider(name string) (Provider, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.providers[name]
	return p, ok
}

// SetStrategy 设置路由策略
func (m *Manager) SetStrategy(s RouterStrategy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.strategy = s
}

// AddRule 添加路由规则
func (m *Manager) AddRule(rule RouteRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append(m.rules, rule)
}

// SetRetryPolicy 设置重试策略
func (m *Manager) SetRetryPolicy(count int, wait, maxWait time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retryCount = count
	m.retryWait = wait
	m.retryMaxWait = maxWait
}

// Chat 执行非流式聊天请求
func (m *Manager) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	provider, err := m.resolveProvider(req.Model)
	if err != nil {
		return nil, err
	}

	resp, err := m.withRetryChat(ctx, func(ctx context.Context) (*ChatResponse, error) {
		return provider.Chat(ctx, req)
	})
	if err != nil && m.strategy == StrategyFallback {
		rule := m.findRule(req.Model)
		if rule != nil && rule.Fallback != "" {
			if fallback, ok := m.GetProvider(rule.Fallback); ok {
				return fallback.Chat(ctx, req)
			}
		}
	}
	return resp, err
}

// ChatStream 执行流式聊天请求
func (m *Manager) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatStreamChunk, error) {
	provider, err := m.resolveProvider(req.Model)
	if err != nil {
		return nil, err
	}
	req.Stream = true
	return provider.ChatStream(ctx, req)
}

// Embed 执行嵌入向量请求
func (m *Manager) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	provider, err := m.resolveProvider(req.Model)
	if err != nil {
		return nil, err
	}
	return m.withRetryEmbed(ctx, func(ctx context.Context) (*EmbedResponse, error) {
		return provider.Embed(ctx, req)
	})
}

// withRetryChat 带重试逻辑的聊天调用
func (m *Manager) withRetryChat(ctx context.Context, fn func(context.Context) (*ChatResponse, error)) (*ChatResponse, error) {
	m.mu.RLock()
	retries := m.retryCount
	wait := m.retryWait
	maxWait := m.retryMaxWait
	m.mu.RUnlock()

	var lastErr error
	for i := 0; i <= retries; i++ {
		resp, err := fn(ctx)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if i < retries && isRetryable(err) {
			jitter := time.Duration(rand.Int63n(int64(wait)))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait + jitter):
			}
			wait *= 2
			if wait > maxWait {
				wait = maxWait
			}
		}
	}
	return nil, lastErr
}

// withRetryEmbed 带重试逻辑的嵌入调用
func (m *Manager) withRetryEmbed(ctx context.Context, fn func(context.Context) (*EmbedResponse, error)) (*EmbedResponse, error) {
	m.mu.RLock()
	retries := m.retryCount
	wait := m.retryWait
	maxWait := m.retryMaxWait
	m.mu.RUnlock()

	var lastErr error
	for i := 0; i <= retries; i++ {
		resp, err := fn(ctx)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if i < retries && isRetryable(err) {
			jitter := time.Duration(rand.Int63n(int64(wait)))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait + jitter):
			}
			wait *= 2
			if wait > maxWait {
				wait = maxWait
			}
		}
	}
	return nil, lastErr
}

// isRetryable 判断错误是否可重试
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	return true
}

// Models 获取所有提供者的模型列表
func (m *Manager) Models(ctx context.Context) ([]ModelInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var all []ModelInfo
	for _, p := range m.providers {
		models, err := p.Models(ctx)
		if err != nil {
			continue
		}
		all = append(all, models...)
	}
	if all == nil {
		all = []ModelInfo{}
	}
	return all, nil
}

// resolveProvider 根据模型名称解析对应的提供者
func (m *Manager) resolveProvider(model string) (Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule := m.findRule(model)
	if rule != nil && rule.Provider != "" {
		if p, ok := m.providers[rule.Provider]; ok {
			return p, nil
		}
	}

	for _, p := range m.providers {
		models, err := p.Models(context.Background())
		if err == nil {
			for _, mdl := range models {
				if mdl.Name == model {
					return p, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("ai: no provider found for model %s: %w", model, ErrModelNotFound)
}

// findRule 查找匹配模型的路由规则
func (m *Manager) findRule(model string) *RouteRule {
	for _, r := range m.rules {
		if r.Model == model {
			return &r
		}
	}
	return nil
}

// CloseAll 关闭所有已注册的提供者
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.providers {
		_ = p.Close()
	}
}

// StreamToString 将流式响应收集为完整字符串
func StreamToString(ctx context.Context, stream <-chan ChatStreamChunk) (string, error) {
	var result string
	for {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case chunk, ok := <-stream:
			if !ok {
				return result, nil
			}
			if chunk.Done {
				return result, nil
			}
			result += chunk.Content
		}
	}
}

// CollectStream 遍历流式响应并对每个块执行回调
func CollectStream(ctx context.Context, stream <-chan ChatStreamChunk, fn func(chunk ChatStreamChunk) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-stream:
			if !ok {
				return nil
			}
			if chunk.Done {
				return nil
			}
			if err := fn(chunk); err != nil {
				return err
			}
		}
	}
}

func init() { _ = io.Discard }
