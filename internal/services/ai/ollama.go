package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sync"

	"github.com/ollama/ollama/api"
)

// OllamaProvider Ollama 本地 LLM 提供者
type OllamaProvider struct {
	client *api.Client // Ollama API 客户端
	host   string      // Ollama 服务地址
	mu     sync.RWMutex
	models []ModelInfo // 缓存的模型列表
}

// NewOllamaProvider 创建 Ollama 提供者
func NewOllamaProvider(host string) (*OllamaProvider, error) {
	u, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("ai: invalid ollama host %s: %w", host, err)
	}
	client := api.NewClient(u, nil)
	return &OllamaProvider{
		client: client,
		host:   host,
	}, nil
}

// Name 返回提供者名称
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// Chat 执行非流式对话请求
func (p *OllamaProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	msgs := make([]api.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, api.Message{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	model := req.Model
	if model == "" {
		model = "llama3"
	}

	var respContent string
	var respUsage Usage

	ollamaReq := &api.ChatRequest{
		Model:    model,
		Messages: msgs,
	}

	err := p.client.Chat(ctx, ollamaReq, func(response api.ChatResponse) error {
		respContent += response.Message.Content
		respUsage.PromptTokens = response.PromptEvalCount
		respUsage.CompletionTokens = response.EvalCount
		respUsage.TotalTokens = response.PromptEvalCount + response.EvalCount
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ai: ollama chat failed: %w", err)
	}

	return &ChatResponse{
		Message: Message{Role: RoleAssistant, Content: respContent},
		Model:   model,
		Usage:   respUsage,
	}, nil
}

// ChatStream 执行流式对话请求
func (p *OllamaProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatStreamChunk, error) {
	msgs := make([]api.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, api.Message{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	model := req.Model
	if model == "" {
		model = "llama3"
	}

	ollamaReq := &api.ChatRequest{
		Model:    model,
		Messages: msgs,
	}

	ch := make(chan ChatStreamChunk, 64)
	go func() {
		defer close(ch)
		err := p.client.Chat(ctx, ollamaReq, func(response api.ChatResponse) error {
			if response.Done {
				ch <- ChatStreamChunk{
					Content: response.Message.Content,
					Done:    true,
					Usage: Usage{
						PromptTokens:     response.PromptEvalCount,
						CompletionTokens: response.EvalCount,
						TotalTokens:      response.PromptEvalCount + response.EvalCount,
					},
				}
				return io.EOF
			}
			ch <- ChatStreamChunk{
				Content: response.Message.Content,
				Done:    false,
			}
			return nil
		})
		if err != nil && !errors.Is(err, io.EOF) {
			ch <- ChatStreamChunk{Content: fmt.Sprintf("error: %v", err), Done: true}
		}
	}()
	return ch, nil
}

// Embed 获取文本嵌入向量
func (p *OllamaProvider) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	model := req.Model
	if model == "" {
		model = "nomic-embed-text"
	}

	var allEmbeddings [][]float64
	for _, input := range req.Inputs {
		embedReq := &api.EmbedRequest{
			Model: model,
			Input: input,
		}
		resp, err := p.client.Embed(ctx, embedReq)
		if err != nil {
			return nil, fmt.Errorf("ai: ollama embed failed: %w", err)
		}
		for _, emb := range resp.Embeddings {
			f64 := make([]float64, len(emb))
			for i, v := range emb {
				f64[i] = float64(v)
			}
			allEmbeddings = append(allEmbeddings, f64)
		}
	}

	return &EmbedResponse{
		Embeddings: allEmbeddings,
		Model:      model,
	}, nil
}

// Models 获取可用模型列表
func (p *OllamaProvider) Models(ctx context.Context) ([]ModelInfo, error) {
	p.mu.RLock()
	if p.models != nil {
		defer p.mu.RUnlock()
		return p.models, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	resp, err := p.client.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("ai: ollama list models failed: %w", err)
	}

	models := make([]ModelInfo, 0, len(resp.Models))
	for _, m := range resp.Models {
		modelType := "chat"
		if containsEmbedModel(m.Name) {
			modelType = "embedding"
		}
		models = append(models, ModelInfo{
			Name:     m.Name,
			Provider: "ollama",
			Type:     modelType,
		})
	}
	p.models = models
	return models, nil
}

// Close 关闭 Ollama 提供者
func (p *OllamaProvider) Close() error {
	return nil
}

// containsEmbedModel 检查模型名是否包含嵌入相关关键字
func containsEmbedModel(name string) bool {
	embedModels := []string{"embed", "nomic", "ada"}
	for _, s := range embedModels {
		if contains(name, s) {
			return true
		}
	}
	return false
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstring(s, substr)
}

// containsSubstring 子串查找实现
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
