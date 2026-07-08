package logger

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type Level int8

const (
	DebugLevel = Level(zerolog.DebugLevel)
	InfoLevel  = Level(zerolog.InfoLevel)
	WarnLevel  = Level(zerolog.WarnLevel)
	ErrorLevel = Level(zerolog.ErrorLevel)
	FatalLevel = Level(zerolog.FatalLevel)
)

type Logger struct {
	zerolog.Logger
	level   Level
	closers []io.Closer
}

var L *Logger

func init() {
	L = New(Config{Level: "info", Format: "json", Output: "stdout"})
}

type Config struct {
	Level      string `json:"level"`
	Format     string `json:"format"`
	Output     string `json:"output"`
	File       string `json:"file,omitempty"`         // 旧版日志文件路径
	LogDir     string `json:"log_dir,omitempty"`      // 日志目录（旋转模式）
	MaxSizeMB  int    `json:"max_size_mb,omitempty"`  // 单文件最大 MB
	MaxAgeDays int    `json:"max_age_days,omitempty"` // 保留天数
}

func New(cfg Config) *Logger {
	return NewWithWriter(cfg, os.Stdout)
}

func NewWithWriter(cfg Config, w io.Writer) *Logger {
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}

	var writers []io.Writer
	var closers []io.Closer

	switch cfg.Output {
	case "rotate":
		prefix := "app"
		if cfg.File != "" {
			prefix = filepath.Base(cfg.File)
			prefix = prefix[:len(prefix)-len(filepath.Ext(prefix))]
		}
		maxSize := int64(cfg.MaxSizeMB) * 1024 * 1024
		if maxSize <= 0 {
			maxSize = 100 * 1024 * 1024
		}
		rw := NewRotateWriter(RotateConfig{
			Dir:     cfg.LogDir,
			Prefix:  prefix,
			MaxSize: maxSize,
			MaxAge:  cfg.MaxAgeDays,
		})
		writers = append(writers, rw)
		closers = append(closers, rw)
	case "file":
		if cfg.File != "" {
			dir := filepath.Dir(cfg.File)
			os.MkdirAll(dir, 0o755)
			maxSize := int64(cfg.MaxSizeMB) * 1024 * 1024
			if maxSize <= 0 {
				maxSize = 100 * 1024 * 1024
			}
			rw := NewRotateWriter(RotateConfig{
				Dir:     dir,
				Prefix:  strings.TrimSuffix(filepath.Base(cfg.File), filepath.Ext(cfg.File)),
				MaxSize: maxSize,
				MaxAge:  cfg.MaxAgeDays,
			})
			writers = append(writers, rw)
			closers = append(closers, rw)
		}
		fallthrough
	default:
		writers = append(writers, w)
	}

	var output io.Writer
	if cfg.Format == "text" {
		output = zerolog.ConsoleWriter{Out: io.MultiWriter(writers...), TimeFormat: time.DateTime}
	} else {
		output = io.MultiWriter(writers...)
	}

	zl := zerolog.New(output).Level(level).With().Timestamp().Caller().Logger()

	return &Logger{Logger: zl, level: Level(level), closers: closers}
}

func (l *Logger) Close() error {
	var errs []error
	for _, c := range l.closers {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	l.closers = nil
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func SetGlobal(l *Logger) {
	L = l
}

func Debug() *zerolog.Event {
	return L.Debug()
}

func Info() *zerolog.Event {
	return L.Info()
}

func Warn() *zerolog.Event {
	return L.Warn()
}

func Error() *zerolog.Event {
	return L.Error()
}

func Fatal() *zerolog.Event {
	return L.Fatal()
}

func WithContext(ctx context.Context) *zerolog.Logger {
	return zerolog.Ctx(ctx)
}

func (l *Logger) SetLevel(level Level) {
	l.level = level
	l.Logger = l.Level(zerolog.Level(level))
}
