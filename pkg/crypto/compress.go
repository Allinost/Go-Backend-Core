package crypto

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

// Compression 数据压缩接口，支持压缩和解压操作
type Compression interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
}

// GzipCompression Gzip 压缩实现
type GzipCompression struct {
	level int // 压缩级别
}

// NewGzipCompression 创建指定压缩级别的 GzipCompression，无效级别自动回退到默认值
func NewGzipCompression(level int) *GzipCompression {
	if level < gzip.HuffmanOnly || level > gzip.BestCompression {
		level = gzip.DefaultCompression
	}
	return &GzipCompression{level: level}
}

// Compress 使用 gzip 压缩数据
func (c *GzipCompression) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, c.level)
	if err != nil {
		return nil, fmt.Errorf("crypto: gzip writer 初始化失败: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("crypto: gzip 压缩失败: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("crypto: gzip 关闭失败: %w", err)
	}
	return buf.Bytes(), nil
}

// Decompress 使用 gzip 解压数据
func (c *GzipCompression) Decompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("crypto: gzip reader 初始化失败: %w", err)
	}
	defer r.Close()
	result, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("crypto: gzip 解压失败: %w", err)
	}
	return result, nil
}

// EncryptCompress 先压缩后加密
func EncryptCompress(cph Cipher, comp Compression, plaintext []byte) ([]byte, error) {
	compressed, err := comp.Compress(plaintext)
	if err != nil {
		return nil, err
	}
	return cph.Encrypt(compressed)
}

// DecryptDecompress 先解密后解压
func DecryptDecompress(cph Cipher, comp Compression, ciphertext []byte) ([]byte, error) {
	decrypted, err := cph.Decrypt(ciphertext)
	if err != nil {
		return nil, err
	}
	return comp.Decompress(decrypted)
}
