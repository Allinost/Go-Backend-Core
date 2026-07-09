package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	assert.NotNil(t, m)
	assert.Empty(t, m.providers)
}

func TestManager_RegisterProvider(t *testing.T) {
	m := NewManager()
	m.RegisterProvider(&mockProvider{name: "test"})
	p, ok := m.GetProvider("test")
	assert.True(t, ok)
	assert.Equal(t, "test", p.Name())
}

func TestManager_GetProviderNotFound(t *testing.T) {
	m := NewManager()
	_, ok := m.GetProvider("nonexistent")
	assert.False(t, ok)
}

func TestManager_Models(t *testing.T) {
	m := NewManager()
	m.RegisterProvider(&mockProvider{name: "p1", models: []ModelInfo{{Name: "model-a", Provider: "p1"}}})
	m.RegisterProvider(&mockProvider{name: "p2", models: []ModelInfo{{Name: "model-b", Provider: "p2"}}})

	models, err := m.Models(context.Background())
	require.NoError(t, err)
	assert.Len(t, models, 2)
}

func TestManager_ModelsEmpty(t *testing.T) {
	m := NewManager()
	models, err := m.Models(context.Background())
	require.NoError(t, err)
	assert.Empty(t, models)
}

func TestManager_ChatNoProvider(t *testing.T) {
	m := NewManager()
	_, err := m.Chat(context.Background(), ChatRequest{Model: "unknown"})
	assert.Error(t, err)
}

func TestManager_SetStrategy(t *testing.T) {
	m := NewManager()
	m.SetStrategy(StrategyFallback)
	assert.Equal(t, StrategyFallback, m.strategy)
}

func TestRouteRule(t *testing.T) {
	m := NewManager()
	m.AddRule(RouteRule{Model: "gpt-4", Provider: "openai", Fallback: "ollama"})
	m.RegisterProvider(&mockProvider{name: "openai", models: []ModelInfo{{Name: "gpt-4", Provider: "openai"}}})

	provider, err := m.resolveProvider("gpt-4")
	require.NoError(t, err)
	assert.Equal(t, "openai", provider.Name())
}

func TestStreamToString(t *testing.T) {
	ch := make(chan ChatStreamChunk, 3)
	ch <- ChatStreamChunk{Content: "Hello"}
	ch <- ChatStreamChunk{Content: " World"}
	ch <- ChatStreamChunk{Content: "", Done: true}
	close(ch)

	result, err := StreamToString(context.Background(), ch)
	require.NoError(t, err)
	assert.Equal(t, "Hello World", result)
}

func TestCollectStream(t *testing.T) {
	ch := make(chan ChatStreamChunk, 3)
	ch <- ChatStreamChunk{Content: "a"}
	ch <- ChatStreamChunk{Content: "b"}
	ch <- ChatStreamChunk{Done: true}
	close(ch)

	var collected string
	err := CollectStream(context.Background(), ch, func(c ChatStreamChunk) error {
		collected += c.Content
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "ab", collected)
}

func TestUsage(t *testing.T) {
	u := Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}
	assert.Equal(t, 10, u.PromptTokens)
	assert.Equal(t, 20, u.CompletionTokens)
	assert.Equal(t, 30, u.TotalTokens)
}

type mockProvider struct {
	name   string
	models []ModelInfo
	err    error
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &ChatResponse{Message: Message{Role: RoleAssistant, Content: "mock reply"}, Model: req.Model}, nil
}

func (m *mockProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatStreamChunk, error) {
	ch := make(chan ChatStreamChunk, 2)
	ch <- ChatStreamChunk{Content: "mock"}
	ch <- ChatStreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func (m *mockProvider) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	return &EmbedResponse{
		Embeddings: [][]float64{{0.1, 0.2, 0.3}},
		Model:      req.Model,
	}, nil
}

func (m *mockProvider) Models(ctx context.Context) ([]ModelInfo, error) {
	return m.models, nil
}

func (m *mockProvider) Close() error { return nil }
