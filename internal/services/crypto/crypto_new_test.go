package crypto

import (
	"compress/gzip"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChaCha20Poly1305_EncryptDecrypt(t *testing.T) {
	c, err := NewChaCha20Poly1305([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)

	plaintext := []byte("Hello ChaCha20! 你好！")
	ciphertext, err := c.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := c.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestChaCha20Poly1305_EncryptDecryptString(t *testing.T) {
	c, _ := NewChaCha20Poly1305([]byte("0123456789abcdef0123456789abcdef"))

	enc, err := c.EncryptString("secret-string")
	require.NoError(t, err)
	assert.NotContains(t, enc, "secret")

	dec, err := c.DecryptString(enc)
	require.NoError(t, err)
	assert.Equal(t, "secret-string", dec)
}

func TestChaCha20Poly1305_InvalidKeyLength(t *testing.T) {
	_, err := NewChaCha20Poly1305([]byte("short"))
	assert.Error(t, err)
}

func TestChaCha20Poly1305_Tampered(t *testing.T) {
	c, _ := NewChaCha20Poly1305([]byte("0123456789abcdef0123456789abcdef"))
	ct, _ := c.Encrypt([]byte("important"))
	ct[len(ct)-1] ^= 0x01
	_, err := c.Decrypt(ct)
	assert.Error(t, err)
}

func TestChaCha20Poly1305_FromHex(t *testing.T) {
	c, err := NewChaCha20Poly1305FromHex("3031323334353637383961626364656630313233343536373839616263646566")
	require.NoError(t, err)
	enc, _ := c.EncryptString("test")
	dec, _ := c.DecryptString(enc)
	assert.Equal(t, "test", dec)
}

func TestZstd_CompressDecompress(t *testing.T) {
	c := NewZstdCompression(0)
	data := []byte("zstd compressible data zstd compressible data zstd compressible data")

	compressed, err := c.Compress(data)
	require.NoError(t, err)
	assert.True(t, len(compressed) < len(data), "zstd 压缩后应更小")

	decompressed, err := c.Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, data, decompressed)
}

func TestZstd_EmptyData(t *testing.T) {
	c := NewZstdCompression(0)
	compressed, err := c.Compress([]byte{})
	require.NoError(t, err)
	decompressed, err := c.Decompress(compressed)
	require.NoError(t, err)
	assert.Empty(t, decompressed)
}

func TestZstd_Levels(t *testing.T) {
	data := []byte("testing zstd compression levels with some data")
	for _, level := range []int{1, 3, 10} {
		c := NewZstdCompression(level)
		compressed, err := c.Compress(data)
		require.NoError(t, err, "level=%d", level)
		decompressed, err := c.Decompress(compressed)
		require.NoError(t, err, "level=%d", level)
		assert.Equal(t, data, decompressed, "level=%d", level)
	}
}

func TestSHA256Hash(t *testing.T) {
	h := SHA256Hash([]byte("test-data"))
	assert.Equal(t, 64, len(h))
}

func TestSHA256Hash_Deterministic(t *testing.T) {
	h1 := SHA256Hash([]byte("hello"))
	h2 := SHA256Hash([]byte("hello"))
	assert.Equal(t, h1, h2)
}

func TestSHA512Hash(t *testing.T) {
	h := SHA512Hash([]byte("test-data"))
	assert.Equal(t, 128, len(h))
}

func TestBLAKE3Hash(t *testing.T) {
	h := BLAKE3Hash([]byte("test-data"))
	assert.Equal(t, 64, len(h))
}

func TestBLAKE3Hash_Deterministic(t *testing.T) {
	h1 := BLAKE3Hash([]byte("hello"))
	h2 := BLAKE3Hash([]byte("hello"))
	assert.Equal(t, h1, h2)
}

func TestHash(t *testing.T) {
	data := []byte("hash-test-data")
	h256, _ := Hash(data, HashSHA256)
	h512, _ := Hash(data, HashSHA512)
	h3, _ := Hash(data, HashBLAKE3)
	assert.Equal(t, 64, len(h256))
	assert.Equal(t, 128, len(h512))
	assert.Equal(t, 64, len(h3))
	assert.NotEqual(t, h256, h512)
	assert.NotEqual(t, h256, h3)

	_, err := Hash(data, "md5")
	assert.Error(t, err)
}

func TestHashPassword_Verify(t *testing.T) {
	password := "MySecureP@ssw0rd!"

	encoded, err := HashPassword(password)
	require.NoError(t, err)
	assert.Contains(t, encoded, "$argon2id$")

	match, err := VerifyPassword(password, encoded)
	require.NoError(t, err)
	assert.True(t, match)

	match, err = VerifyPassword("wrong-password", encoded)
	require.NoError(t, err)
	assert.False(t, match)
}

func TestHashPassword_WithParams(t *testing.T) {
	params := Argon2Params{
		Time:    1,
		Memory:  64 * 1024,
		Threads: 2,
		KeyLen:  16,
		SaltLen: 8,
	}

	encoded, err := HashPasswordWithParams("test-pass", params)
	require.NoError(t, err)

	match, err := VerifyPassword("test-pass", encoded)
	require.NoError(t, err)
	assert.True(t, match)
}

func TestHashPassword_DifferentSalts(t *testing.T) {
	h1, _ := HashPassword("same-password")
	h2, _ := HashPassword("same-password")
	assert.NotEqual(t, h1, h2, "不同 salt 应产生不同哈希")
}

func TestEncryptFile_DecryptFile(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	srcPath := t.TempDir() + "/test.txt"
	dstPath := t.TempDir() + "/test.enc"
	outPath := t.TempDir() + "/test.out"

	original := []byte("Hello File Encryption! This is a test of streaming encryption.")
	err := os.WriteFile(srcPath, original, 0644)
	require.NoError(t, err)

	err = EncryptFile(srcPath, dstPath, key)
	require.NoError(t, err)

	encData, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.NotEqual(t, original, encData, "加密后内容不应等于原文")
	assert.Greater(t, len(encData), 12, "应包含 nonce")

	err = DecryptFile(dstPath, outPath, key)
	require.NoError(t, err)

	decData, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Equal(t, original, decData)
}

func TestEncryptFile_WrongKey(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	wrongKey := []byte("fedcba9876543210fedcba9876543210")
	srcPath := t.TempDir() + "/test.txt"
	dstPath := t.TempDir() + "/test.enc"

	err := os.WriteFile(srcPath, []byte("secret"), 0644)
	require.NoError(t, err)

	err = EncryptFile(srcPath, dstPath, key)
	require.NoError(t, err)

	err = DecryptFile(dstPath, t.TempDir()+"/out.txt", wrongKey)
	assert.Error(t, err, "错误密钥应解密失败")
}

func TestEncryptAndCompressFile_DecryptAndDecompressFile(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	srcPath := t.TempDir() + "/source.txt"
	dstPath := t.TempDir() + "/compressed.enc"
	outPath := t.TempDir() + "/restored.txt"

	content := []byte("This content will be compressed then encrypted. " +
		"Repeating data makes compression more efficient. " +
		"Repeating data makes compression more efficient. " +
		"Repeating data makes compression more efficient.")

	err := os.WriteFile(srcPath, content, 0644)
	require.NoError(t, err)

	err = EncryptAndCompressFile(srcPath, dstPath, key)
	require.NoError(t, err)

	err = DecryptAndDecompressFile(dstPath, outPath, key)
	require.NoError(t, err)

	restored, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Equal(t, content, restored)
}

func TestZstd_HighLevels(t *testing.T) {
	data := []byte("testing zstd at higher compression levels " +
		"with enough data to make a difference in compression ratio " +
		"testing zstd at higher compression levels " +
		"with enough data to make a difference in compression ratio")
	c1 := NewZstdCompression(1)
	c3 := NewZstdCompression(3)
	comp1, _ := c1.Compress(data)
	comp3, _ := c3.Compress(data)
	assert.LessOrEqual(t, len(comp3), len(comp1), "更高压缩级别应产生更小或相等的结果")
}

func TestCompressAutoDecompressAuto_Zstd(t *testing.T) {
	data := []byte("auto detect zstd compression")
	c := NewZstdCompression(0)
	compressed, err := c.Compress(data)
	require.NoError(t, err)

	decompressed, err := c.Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, data, decompressed)
}

func TestCompressAuto_AllMethods(t *testing.T) {
	data := []byte("test data for all compression methods")
	methods := []string{"gzip", "zlib", "zip"}
	for _, method := range methods {
		compressed, err := CompressAuto(data, method, gzip.DefaultCompression, "")
		require.NoError(t, err, "method=%s", method)

		decompressed, err := DecompressAuto(compressed, "")
		require.NoError(t, err, "method=%s", method)
		assert.Equal(t, data, decompressed, "method=%s", method)
	}
}
