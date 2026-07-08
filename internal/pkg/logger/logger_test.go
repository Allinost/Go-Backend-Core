package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWithWriter_Output(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(Config{Level: "info", Format: "json"}, &buf)
	l.Info().Msg("test msg")
	assert.Contains(t, buf.String(), "test msg")
}

func TestNew_DefaultOutput(t *testing.T) {
	l := New(Config{Level: "debug", Format: "json", Output: "stdout"})
	assert.NotNil(t, l)
}

func TestNew_InvalidLevelDefaultsToInfo(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(Config{Level: "invalid", Format: "json"}, &buf)
	l.Debug().Msg("debug should be filtered")
	l.Info().Msg("info should appear")
	assert.NotContains(t, buf.String(), "debug should be filtered")
	assert.Contains(t, buf.String(), "info should appear")
}

func TestNew_FileOutput(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.log")

	f, err := os.Create(filePath)
	require.NoError(t, err)
	defer f.Close()

	l := NewWithWriter(Config{Level: "info", Format: "json"}, f)
	l.Info().Msg("file log")

	data, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "file log")
}

func TestLevel_Filtering(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(Config{Level: "error", Format: "json"}, &buf)

	l.Debug().Msg("should not appear")
	l.Info().Msg("should not appear")
	l.Error().Msg("should appear")

	assert.NotContains(t, buf.String(), "should not appear")
	assert.Contains(t, buf.String(), "should appear")
}

func TestJSON_Format(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(Config{Level: "info", Format: "json"}, &buf)

	l.Info().Str("key", "value").Msg("json msg")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 1)

	var parsed map[string]interface{}
	err := json.Unmarshal([]byte(lines[0]), &parsed)
	assert.NoError(t, err)
	assert.Equal(t, "info", parsed["level"])
	assert.Equal(t, "json msg", parsed["message"])
	assert.Equal(t, "value", parsed["key"])
	assert.NotEmpty(t, parsed["time"])
	assert.NotEmpty(t, parsed["caller"])
}

func TestText_Format(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(Config{Level: "info", Format: "text"}, &buf)
	l.Info().Msg("text msg")
	assert.Contains(t, buf.String(), "text msg")
}

func TestGlobal_Level(t *testing.T) {
	l := New(Config{Level: "info", Format: "json", Output: "stdout"})
	SetGlobal(l)
	assert.Equal(t, L, l)
}

func TestSetLevel(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(Config{Level: "debug", Format: "json"}, &buf)
	l.SetLevel(ErrorLevel)

	l.Info().Msg("should be filtered")
	assert.Empty(t, buf.String())
}

func TestGlobal_Shorthands(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(Config{Level: "debug", Format: "json"}, &buf)
	SetGlobal(l)

	Debug().Msg("debug via global")
	Info().Msg("info via global")
	Warn().Msg("warn via global")
	Error().Msg("error via global")

	assert.Contains(t, buf.String(), "debug via global")
	assert.Contains(t, buf.String(), "info via global")
	assert.Contains(t, buf.String(), "error via global")
}

func TestWithContext(t *testing.T) {
	l := New(Config{Level: "info", Format: "json", Output: "stdout"})
	SetGlobal(l)

	ctx := WithTraceID(context.Background(), "trace-123")
	logger := WithContext(ctx)
	assert.NotNil(t, logger)
}

type syncBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func TestConcurrent_Access(t *testing.T) {
	var sb syncBuf
	l := NewWithWriter(Config{Level: "info", Format: "json"}, &sb)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			l.Info().Int("n", n).Msg("concurrent")
		}(i)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSpace(sb.String()), "\n")
	assert.Len(t, lines, 20)
}
