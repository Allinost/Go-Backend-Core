package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderTextString(t *testing.T) {
	out, err := RenderTextString("Hello {{.Name}}", map[string]any{"Name": "World"})
	require.NoError(t, err)
	assert.Equal(t, "Hello World", out)
}

func TestRenderHTMLString(t *testing.T) {
	out, err := RenderHTMLString("<b>{{.Name}}</b>", map[string]any{"Name": "World"})
	require.NoError(t, err)
	assert.Equal(t, "<b>World</b>", out)
}

func TestRenderHTMLStringEscaped(t *testing.T) {
	out, err := RenderHTMLString("<b>{{.Name}}</b>", map[string]any{"Name": "<script>"})
	require.NoError(t, err)
	assert.Equal(t, "<b>&lt;script&gt;</b>", out)
}

func TestBuiltinFuncs(t *testing.T) {
	tests := []struct {
		name     string
		tpl      string
		data     any
		expected string
	}{
		{"lower", "{{.V | lower}}", map[string]string{"V": "HELLO"}, "hello"},
		{"upper", "{{.V | upper}}", map[string]string{"V": "hello"}, "HELLO"},
		{"default not nil", "{{.V | default \"fallback\"}}", map[string]string{"V": "val"}, "val"},
		{"default nil", "{{.V | default \"fallback\"}}", map[string]any{"V": nil}, "fallback"},
		{"default empty", `{{.V | default "fallback"}}`, map[string]string{"V": ""}, "fallback"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := RenderTextString(tt.tpl, tt.data)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, out)
		})
	}
}

func TestRenderText_UnknownTemplate(t *testing.T) {
	_, err := RenderText("non-existent", nil)
	assert.Error(t, err)
}

func TestListTemplates_Empty(t *testing.T) {
	e := NewEngineFS(nil)
	assert.Empty(t, e.ListTemplates())
}

func TestTemplateExists(t *testing.T) {
	e := NewEngineFS(nil)
	assert.False(t, e.TemplateExists("anything"))
}

func TestAddFunc(t *testing.T) {
	AddFunc("add", func(a, b int) int { return a + b })
	out, err := RenderTextString("{{add 1 2}}", nil)
	require.NoError(t, err)
	assert.Equal(t, "3", out)
}
