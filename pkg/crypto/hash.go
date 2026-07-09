package crypto

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"

	"lukechampine.com/blake3"
)

func SHA256Hash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func SHA512Hash(data []byte) string {
	h := sha512.Sum512(data)
	return hex.EncodeToString(h[:])
}

func BLAKE3Hash(data []byte) string {
	h := blake3.Sum256(data)
	return hex.EncodeToString(h[:])
}

type HashAlgo string

const (
	HashSHA256 HashAlgo = "sha256"
	HashSHA512 HashAlgo = "sha512"
	HashBLAKE3 HashAlgo = "blake3"
)

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
