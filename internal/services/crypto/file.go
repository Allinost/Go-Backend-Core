package crypto

import (
	"bufio"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

const (
	fileBufSize     = 1 * 1024 * 1024
	nonceBaseLen    = 8
	nonceCounterLen = 4
	fileNonceLen    = nonceBaseLen + nonceCounterLen
)

type StreamCipher interface {
	NewEncryptingWriter(dst io.Writer) (io.WriteCloser, error)
	NewDecryptingReader(src io.Reader) (io.Reader, error)
}

type aesGCMStream struct {
	key []byte
}

func newAESGCMStream(key []byte) *aesGCMStream {
	return &aesGCMStream{key: key}
}

func (s *aesGCMStream) NewEncryptingWriter(dst io.Writer) (io.WriteCloser, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, fmt.Errorf("crypto: AES 初始化失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: GCM 初始化失败: %w", err)
	}

	base := make([]byte, nonceBaseLen)
	if _, err := io.ReadFull(rand.Reader, base); err != nil {
		return nil, fmt.Errorf("crypto: 随机数生成失败: %w", err)
	}
	if _, err := dst.Write(base); err != nil {
		return nil, fmt.Errorf("crypto: 写入 nonce base 失败: %w", err)
	}

	return &encryptWriter{gcm: gcm, base: base, counter: 0, dst: dst}, nil
}

func (s *aesGCMStream) NewDecryptingReader(src io.Reader) (io.Reader, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, fmt.Errorf("crypto: AES 初始化失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: GCM 初始化失败: %w", err)
	}

	base := make([]byte, nonceBaseLen)
	if _, err := io.ReadFull(src, base); err != nil {
		return nil, fmt.Errorf("crypto: 读取 nonce base 失败: %w", err)
	}

	return &decryptReader{gcm: gcm, base: base, src: src}, nil
}

func buildNonce(base []byte, counter uint64) []byte {
	nonce := make([]byte, fileNonceLen)
	copy(nonce, base)
	binary.BigEndian.PutUint32(nonce[nonceBaseLen:], uint32(counter))
	return nonce
}

type encryptWriter struct {
	gcm     cipher.AEAD
	base    []byte
	counter uint64
	dst     io.Writer
}

func (w *encryptWriter) Write(data []byte) (int, error) {
	nonce := buildNonce(w.base, w.counter)
	w.counter++
	ct := w.gcm.Seal(nil, nonce, data, nil)
	if _, err := w.dst.Write(ct); err != nil {
		return 0, err
	}
	return len(data), nil
}

func (w *encryptWriter) Close() error {
	return nil
}

type decryptReader struct {
	gcm     cipher.AEAD
	base    []byte
	counter uint64
	src     io.Reader
}

func (r *decryptReader) Read(data []byte) (int, error) {
	tmp := make([]byte, len(data)+16)
	n, err := r.src.Read(tmp)
	if err != nil && err != io.EOF {
		return 0, err
	}
	if n == 0 {
		return 0, io.EOF
	}

	nonce := buildNonce(r.base, r.counter)
	r.counter++
	pt, err := r.gcm.Open(nil, nonce, tmp[:n], nil)
	if err != nil {
		return 0, fmt.Errorf("crypto: 流式解密失败: %w", err)
	}
	return copy(data, pt), nil
}

func EncryptFile(srcPath, dstPath string, key []byte) error {
	_, err := NewAESGCM(key)
	if err != nil {
		return err
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("crypto: 打开源文件失败: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("crypto: 创建目标文件失败: %w", err)
	}
	defer dst.Close()

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("crypto: AES 初始化失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("crypto: GCM 初始化失败: %w", err)
	}

	base := make([]byte, nonceBaseLen)
	if _, err := io.ReadFull(rand.Reader, base); err != nil {
		return fmt.Errorf("crypto: 随机数生成失败: %w", err)
	}

	writer := bufio.NewWriterSize(dst, fileBufSize)
	defer writer.Flush()

	if _, err := writer.Write(base); err != nil {
		return fmt.Errorf("crypto: 写入 nonce base 失败: %w", err)
	}

	reader := bufio.NewReaderSize(src, fileBufSize)
	buf := make([]byte, fileBufSize)
	var counter uint64
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			nonce := buildNonce(base, counter)
			counter++
			ct := gcm.Seal(nil, nonce, buf[:n], nil)
			if _, werr := writer.Write(ct); werr != nil {
				return fmt.Errorf("crypto: 写入加密数据失败: %w", werr)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("crypto: 读取源文件失败: %w", err)
		}
	}

	return nil
}

func DecryptFile(srcPath, dstPath string, key []byte) error {
	_, err := NewAESGCM(key)
	if err != nil {
		return err
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("crypto: 打开加密文件失败: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("crypto: 创建目标文件失败: %w", err)
	}
	defer dst.Close()

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("crypto: AES 初始化失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("crypto: GCM 初始化失败: %w", err)
	}

	reader := bufio.NewReaderSize(src, fileBufSize)
	base := make([]byte, nonceBaseLen)
	if _, err := io.ReadFull(reader, base); err != nil {
		return fmt.Errorf("crypto: 读取 nonce base 失败: %w", err)
	}

	writer := bufio.NewWriterSize(dst, fileBufSize)
	defer writer.Flush()

	buf := make([]byte, fileBufSize+gcm.Overhead())
	var counter uint64
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			nonce := buildNonce(base, counter)
			counter++
			pt, decErr := gcm.Open(nil, nonce, buf[:n], nil)
			if decErr != nil {
				return fmt.Errorf("crypto: 解密数据失败: %w", decErr)
			}
			if _, writeErr := writer.Write(pt); writeErr != nil {
				return fmt.Errorf("crypto: 写入解密数据失败: %w", writeErr)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("crypto: 读取加密文件失败: %w", err)
		}
	}

	return nil
}

func EncryptAndCompressFile(srcPath, dstPath string, key []byte) error {
	srcData, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("crypto: 读取源文件失败: %w", err)
	}

	gz := NewGzipCompression(gzip.DefaultCompression)
	compressed, err := gz.Compress(srcData)
	if err != nil {
		return fmt.Errorf("crypto: 压缩失败: %w", err)
	}

	tmpPath := srcPath + ".tmp.enc"
	if err := os.WriteFile(tmpPath, compressed, 0644); err != nil {
		return fmt.Errorf("crypto: 写入临时文件失败: %w", err)
	}
	defer os.Remove(tmpPath)

	return EncryptFile(tmpPath, dstPath, key)
}

func DecryptAndDecompressFile(srcPath, dstPath string, key []byte) error {
	tmpPath := srcPath + ".tmp.gz"
	if err := DecryptFile(srcPath, tmpPath, key); err != nil {
		os.Remove(tmpPath)
		return err
	}
	defer os.Remove(tmpPath)

	compressed, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("crypto: 读取临时文件失败: %w", err)
	}

	gz := NewGzipCompression(gzip.DefaultCompression)
	decompressed, err := gz.Decompress(compressed)
	if err != nil {
		return fmt.Errorf("crypto: 解压失败: %w", err)
	}

	return os.WriteFile(dstPath, decompressed, 0644)
}

func DecryptWithGCM(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: AES 初始化失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: GCM 初始化失败: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("crypto: 密文太短")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
