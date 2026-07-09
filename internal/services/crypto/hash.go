package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
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

type Argon2Params struct {
	Time    uint32
	Memory  uint32
	Threads uint8
	KeyLen  uint32
	SaltLen uint32
}

var DefaultArgon2Params = Argon2Params{
	Time:    3,
	Memory:  64 * 1024,
	Threads: 4,
	KeyLen:  32,
	SaltLen: 16,
}

const argon2Prefix = "$argon2id$"

func HashPassword(password string) (string, error) {
	return HashPasswordWithParams(password, DefaultArgon2Params)
}

func HashPasswordWithParams(password string, params Argon2Params) (string, error) {
	salt := make([]byte, params.SaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("crypto: salt 生成失败: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, params.Time, params.Memory, params.Threads, params.KeyLen)

	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	hashB64 := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf("%sv=%d,m=%d,t=%d,p=%d$%s$%s",
		argon2Prefix, params.KeyLen*8, params.Memory*1024, params.Time, params.Threads, saltB64, hashB64)

	return encoded, nil
}

func VerifyPassword(password, encodedHash string) (bool, error) {
	params, salt, hash, err := decodeArgon2Hash(encodedHash)
	if err != nil {
		return false, err
	}

	computed := argon2.IDKey([]byte(password), salt, params.Time, params.Memory, params.Threads, params.KeyLen)

	if len(computed) != len(hash) {
		return false, nil
	}
	for i := range computed {
		if computed[i] != hash[i] {
			return false, nil
		}
	}
	return true, nil
}

func decodeArgon2Hash(encoded string) (Argon2Params, []byte, []byte, error) {
	if !strings.HasPrefix(encoded, argon2Prefix) {
		return Argon2Params{}, nil, nil, fmt.Errorf("crypto: 无效的 Argon2 哈希格式")
	}

	parts := strings.Split(encoded[len(argon2Prefix):], "$")
	if len(parts) != 3 {
		return Argon2Params{}, nil, nil, fmt.Errorf("crypto: Argon2 哈希段数错误")
	}

	var params Argon2Params
	var keyLenBits uint32
	var memoryKiB uint32
	_, err := fmt.Sscanf(parts[0], "v=%d,m=%d,t=%d,p=%d",
		&keyLenBits, &memoryKiB, &params.Time, &params.Threads)
	if err != nil {
		return Argon2Params{}, nil, nil, fmt.Errorf("crypto: 解析 Argon2 参数失败: %w", err)
	}
	params.KeyLen = keyLenBits / 8
	params.Memory = memoryKiB / 1024

	salt, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return Argon2Params{}, nil, nil, fmt.Errorf("crypto: 解码 salt 失败: %w", err)
	}
	params.SaltLen = uint32(len(salt))

	hash, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return Argon2Params{}, nil, nil, fmt.Errorf("crypto: 解码哈希失败: %w", err)
	}

	return params, salt, hash, nil
}
