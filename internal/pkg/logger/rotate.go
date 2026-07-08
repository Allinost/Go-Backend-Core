package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type RotateConfig struct {
	Dir       string // 日志目录
	Prefix    string // 文件名前缀
	MaxSize   int64  // 单文件最大字节（默认 100MB）
	MaxAge    int    // 保留天数（默认 30）
	MaxBackup int    // 单日最大备份数（默认 10）
}

type dailyWriter struct {
	cfg    RotateConfig
	mu     sync.Mutex
	file   *os.File
	date   string
	size   int64
	index  int32
	closed atomic.Bool
}

func NewRotateWriter(cfg RotateConfig) io.WriteCloser {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 100 * 1024 * 1024
	}
	if cfg.MaxAge <= 0 {
		cfg.MaxAge = 30
	}
	if cfg.MaxBackup <= 0 {
		cfg.MaxBackup = 10
	}
	if cfg.Dir == "" {
		cfg.Dir = "logs"
	}

	w := &dailyWriter{cfg: cfg}
	w.open()

	go w.cleanupLoop()

	return w
}

func (w *dailyWriter) open() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.openLocked()
}

func (w *dailyWriter) openLocked() {
	now := time.Now()
	date := now.Format("2006-01-02")

	dir := w.cfg.Dir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}

	path := filepath.Join(dir, fmt.Sprintf("%s-%s.log", w.cfg.Prefix, date))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}

	info, _ := file.Stat()
	w.file = file
	w.date = date
	w.size = info.Size()
	w.index = 0
}

func (w *dailyWriter) rotateLocked() {
	if w.file != nil {
		w.file.Close()
	}

	w.index++

	now := time.Now()
	date := now.Format("2006-01-02")
	dir := w.cfg.Dir
	path := filepath.Join(dir, fmt.Sprintf("%s-%s.%d.log", w.cfg.Prefix, date, w.index))

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		w.openLocked()
		return
	}

	w.file = file
	w.size = 0
}

func (w *dailyWriter) Write(p []byte) (int, error) {
	if w.closed.Load() {
		return 0, fmt.Errorf("writer 已关闭")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	today := now.Format("2006-01-02")
	if today != w.date {
		w.openLocked()
	}

	n, err := w.file.Write(p)
	if err != nil {
		return n, err
	}

	w.size += int64(n)
	if w.size >= w.cfg.MaxSize && w.index < int32(w.cfg.MaxBackup) {
		w.rotateLocked()
	}

	return n, nil
}

func (w *dailyWriter) Close() error {
	if w.closed.Load() {
		return nil
	}
	w.closed.Store(true)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		return err
	}
	return nil
}

func (w *dailyWriter) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		<-ticker.C
		if w.closed.Load() {
			return
		}
		w.cleanup()
	}
}

func (w *dailyWriter) cleanupTest() {
	w.cleanup()
}

func (w *dailyWriter) Cleanup() {
	w.cleanup()
}

func (w *dailyWriter) cleanup() {
	cutoff := time.Now().AddDate(0, 0, -w.cfg.MaxAge)
	dir := w.cfg.Dir

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), w.cfg.Prefix) {
			continue
		}
		files = append(files, e.Name())
	}

	sort.Slice(files, func(i, j int) bool {
		infoI, _ := os.Stat(filepath.Join(dir, files[i]))
		infoJ, _ := os.Stat(filepath.Join(dir, files[j]))
		if infoI == nil || infoJ == nil {
			return files[i] < files[j]
		}
		return infoI.ModTime().Before(infoJ.ModTime())
	})

	for _, f := range files {
		path := filepath.Join(dir, f)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(path)
		}
	}
}
