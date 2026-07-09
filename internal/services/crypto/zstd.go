package crypto

import (
	"bytes"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

type ZstdCompression struct {
	level zstd.EncoderLevel
}

func NewZstdCompression(level int) *ZstdCompression {
	zstdLevel := zstd.EncoderLevel(level)
	if zstdLevel < zstd.SpeedFastest || zstdLevel > zstd.SpeedBestCompression {
		zstdLevel = zstd.SpeedDefault
	}
	return &ZstdCompression{level: zstdLevel}
}

func (c *ZstdCompression) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf, zstd.WithEncoderLevel(c.level))
	if err != nil {
		return nil, fmt.Errorf("crypto: zstd writer 初始化失败: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("crypto: zstd 压缩失败: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("crypto: zstd 关闭失败: %w", err)
	}
	return buf.Bytes(), nil
}

func (c *ZstdCompression) Decompress(data []byte) ([]byte, error) {
	r, err := zstd.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("crypto: zstd reader 初始化失败: %w", err)
	}
	defer r.Close()
	result, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("crypto: zstd 解压失败: %w", err)
	}
	return result, nil
}
