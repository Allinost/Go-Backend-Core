package logger

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRotateWriter_Basic(t *testing.T) {
	dir := t.TempDir()
	w := NewRotateWriter(RotateConfig{
		Dir:       dir,
		Prefix:    "test",
		MaxSize:   1024 * 1024,
		MaxAge:    30,
		MaxBackup: 10,
	})

	n, err := w.Write([]byte("hello world\n"))
	assert.NoError(t, err)
	assert.Greater(t, n, 0)

	w.Close()

	files, _ := os.ReadDir(dir)
	assert.Len(t, files, 1)
	assert.Contains(t, files[0].Name(), "test-")
	assert.Contains(t, files[0].Name(), ".log")
}

func TestRotateWriter_RotateBySize(t *testing.T) {
	dir := t.TempDir()
	w := NewRotateWriter(RotateConfig{
		Dir:       dir,
		Prefix:    "size-test",
		MaxSize:   10,
		MaxAge:    30,
		MaxBackup: 3,
	})

	for i := 0; i < 100; i++ {
		_, err := w.Write([]byte("0123456789\n"))
		require.NoError(t, err)
	}

	w.Close()

	files, _ := os.ReadDir(dir)
	assert.GreaterOrEqual(t, len(files), 2)
}

func TestRotateWriter_CloseMultiple(t *testing.T) {
	dir := t.TempDir()
	w := NewRotateWriter(RotateConfig{
		Dir:    dir,
		Prefix: "close-test",
	})
	assert.NoError(t, w.Close())
	assert.NoError(t, w.Close())
}

func TestRotateWriter_WriteAfterClose(t *testing.T) {
	dir := t.TempDir()
	w := NewRotateWriter(RotateConfig{
		Dir:    dir,
		Prefix: "closed-test",
	})
	w.Close()

	_, err := w.Write([]byte("should fail"))
	assert.Error(t, err)
}

func TestRotateWriter_Concurrent(t *testing.T) {
	dir := t.TempDir()
	w := NewRotateWriter(RotateConfig{
		Dir:       dir,
		Prefix:    "concurrent",
		MaxSize:   50,
		MaxAge:    30,
		MaxBackup: 5,
	})

	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			w.Write([]byte("concurrent write\n"))
		}
		done <- struct{}{}
	}()
	for i := 0; i < 50; i++ {
		w.Write([]byte("concurrent write\n"))
	}
	<-done

	w.Close()

	files, _ := os.ReadDir(dir)
	assert.GreaterOrEqual(t, len(files), 1)
}

func TestLogger_Close(t *testing.T) {
	dir := t.TempDir()
	l := New(Config{
		Level:      "debug",
		Format:     "json",
		Output:     "rotate",
		LogDir:     dir,
		MaxSizeMB:  1,
		MaxAgeDays: 30,
	})

	l.Info().Msg("before close")
	err := l.Close()
	assert.NoError(t, err)
}

func TestLogger_RotateOutput(t *testing.T) {
	dir := t.TempDir()
	l := New(Config{
		Level:      "info",
		Format:     "json",
		Output:     "rotate",
		LogDir:     dir,
		MaxSizeMB:  1,
		MaxAgeDays: 30,
	})

	l.Info().Msg("rotate output test")
	l.Close()

	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
}

func TestLogger_FileOutputWithRotation(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "app.log")

	l := New(Config{
		Level:      "info",
		Format:     "json",
		Output:     "file",
		File:       logFile,
		MaxSizeMB:  1,
		MaxAgeDays: 30,
	})

	l.Info().Msg("file rotation test")
	l.Close()

	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(files), 1)
}

func TestCleanup_RemovesOldFiles(t *testing.T) {
	dir := t.TempDir()

	oldFile := filepath.Join(dir, "test-2020-01-01.log")
	require.NoError(t, os.WriteFile(oldFile, []byte("old log"), 0o644))

	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(oldFile, oldTime, oldTime))

	dw := &dailyWriter{
		cfg: RotateConfig{
			Dir:    dir,
			Prefix: "test",
			MaxAge: 1,
		},
	}
	dw.open()

	dw.Write([]byte("new log\n"))
	dw.cleanupTest()

	files, _ := os.ReadDir(dir)
	assert.Len(t, files, 1)

	dw.Close()
}
