package crypto

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"

	"lukechampine.com/blake3"
)

// SHA256Hash 计算数据的 SHA-256 哈希，返回 hex 编码字符串
func SHA256Hash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// SHA512Hash 计算数据的 SHA-512 哈希，返回 hex 编码字符串
func SHA512Hash(data []byte) string {
	h := sha512.Sum512(data)
	return hex.EncodeToString(h[:])
}

// BLAKE3Hash 计算数据的 BLAKE3 哈希，返回 hex 编码字符串
func BLAKE3Hash(data []byte) string {
	h := blake3.Sum256(data)
	return hex.EncodeToString(h[:])
}

// HashAlgo 支持的哈希算法类型
type HashAlgo string

const (
	HashSHA256 HashAlgo = "sha256" // SHA-256 算法
	HashSHA512 HashAlgo = "sha512" // SHA-512 算法
	HashBLAKE3 HashAlgo = "blake3" // BLAKE3 算法
)

// Hash 根据指定哈希算法计算数据的哈希值
func Hash(data []byte, algo HashAlgo) (string, error) {
	switch algo {
	case HashSHA256:
		return SHA256Hash(data), nil
	case HashSHA512:
		return SHA512Hash(data), nil
	case HashBLAKE3:
		return BLAKE3Hash(data), nil
	default:
		return "", fmt.Errorf("crypto: 不支持的哈希算法 %s", algo)
	}
}
