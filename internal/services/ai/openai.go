package ai

import (
	"context"
	"fmt"
	"sync"

	gopenai "github.com/sashabaranov/go-openai"
)

// OpenAIProvider OpenAI 兼容 API 提供者
type OpenAIProvider struct {
	client *gopenai.Client // OpenAI API 客户端
	host   string          // API 基础地址
	mu     sync.RWMutex
	models []ModelInfo // 缓存的模型列表
}

// NewOpenAIProvider 创建 OpenAI 提供者
func NewOpenAIProvider(apiKey, baseURL string) *OpenAIProvider {
	config := gopenai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	client := gopenai.NewClientWithConfig(config)
	return &OpenAIProvider{
		client: client,
		host:   baseURL,
	}
}

// Name 返回提供者名称
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// Chat 执行非流式对话请求
func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	msgs := make([]gopenai.ChatCompletionMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, gopenai.ChatCompletionMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	model := req.Model
	if model == "" {
		model = "gpt-4o-mini"
	}

	resp, err := p.client.CreateChatCompletion(ctx, gopenai.ChatCompletionRequest{
		Model:    model,
		Messages: msgs,
	})
	if err != nil {
		return nil, fmt.Errorf("ai: openai chat failed: %w", err)
	}

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	return &ChatResponse{
		Message: Message{Role: RoleAssistant, Content: content},
		Model:   model,
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}, nil
}

// ChatStream 执行流式对话请求
func (p *OpenAIProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatStreamChunk, error) {
	msgs := make([]gopenai.ChatCompletionMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, gopenai.ChatCompletionMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	model := req.Model
	if model == "" {
		model = "gpt-4o-mini"
	}

	stream, err := p.client.CreateChatCompletionStream(ctx, gopenai.ChatCompletionRequest{
		Model:    model,
		Messages: msgs,
		Stream:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("ai: openai chat stream failed: %w", err)
	}

	ch := make(chan ChatStreamChunk, 64)
	go func() {
		defer stream.Close()
		defer close(ch)
		for {
			resp, err := stream.Recv()
			if err != nil {
				ch <- ChatStreamChunk{Done: true}
				return
			}
			content := ""
			if len(resp.Choices) > 0 {
				content = resp.Choices[0].Delta.Content
			}
			ch <- ChatStreamChunk{
				Content: content,
				Done:    resp.Choices[0].FinishReason == "stop",
			}
		}
	}()
	return ch, nil
}

// Embed 执行文本嵌入向量请求
func (p *OpenAIProvider) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	model := req.Model
	if model == "" {
		model = "text-embedding-ada-002"
	}

	resp, err := p.client.CreateEmbeddings(ctx, gopenai.EmbeddingRequest{
		Model: gopenai.EmbeddingModel(model),
		Input: req.Inputs,
	})
	if err != nil {
		return nil, fmt.Errorf("ai: openai embed failed: %w", err)
	}

	embeddings := make([][]float64, 0, len(resp.Data))
	for _, e := range resp.Data {
		f64 := make([]float64, len(e.Embedding))
		for i, v := range e.Embedding {
			f64[i] = float64(v)
		}
		embeddings = append(embeddings, f64)
	}

	return &EmbedResponse{
		Embeddings: embeddings,
		Model:      model,
	}, nil
}

// Models 获取可用模型列表
func (p *OpenAIProvider) Models(ctx context.Context) ([]ModelInfo, error) {
	p.mu.RLock()
	if p.models != nil {
		defer p.mu.RUnlock()
		return p.models, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	resp, err := p.client.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("ai: openai list models failed: %w", err)
	}

	models := make([]ModelInfo, 0, len(resp.Models))
	for _, m := range resp.Models {
		modelType := "chat"
		if containsEmbedModel(m.ID) {
			modelType = "embedding"
		}
		models = append(models, ModelInfo{
			Name:     m.ID,
			Provider: "openai",
			Type:     modelType,
		})
	}
	p.models = models
	return models, nil
}

// Close 关闭 OpenAI 提供者
func (p *OpenAIProvider) Close() error {
	return nil
}
