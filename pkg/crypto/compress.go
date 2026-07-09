package crypto

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

type Compression interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
}

type GzipCompression struct {
	level int
}

func NewGzipCompression(level int) *GzipCompression {
	if level < gzip.HuffmanOnly || level > gzip.BestCompression {
		level = gzip.DefaultCompression
	}
	return &GzipCompression{level: level}
}

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

func EncryptCompress(cph Cipher, comp Compression, plaintext []byte) ([]byte, error) {
	compressed, err := comp.Compress(plaintext)
	if err != nil {
		return nil, err
	}
	return cph.Encrypt(compressed)
}

func DecryptDecompress(cph Cipher, comp Compression, ciphertext []byte) ([]byte, error) {
	decrypted, err := cph.Decrypt(ciphertext)
	if err != nil {
		return nil, err
	}
	return comp.Decompress(decrypted)
}
