package crypto

import (
	"compress/gzip"
	"compress/zlib"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAESGCM_EncryptDecrypt(t *testing.T) {
	c, err := NewAESGCM([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)

	plaintext := []byte("Hello, World! 你好，世界！")
	ciphertext, err := c.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := c.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestAESGCM_EncryptDecryptString(t *testing.T) {
	c, _ := NewAESGCM([]byte("0123456789abcdef0123456789abcdef"))

	enc, err := c.EncryptString("secret-data")
	require.NoError(t, err)
	assert.NotContains(t, enc, "secret")

	dec, err := c.DecryptString(enc)
	require.NoError(t, err)
	assert.Equal(t, "secret-data", dec)
}

func TestAESGCM_DifferentNonce(t *testing.T) {
	c, _ := NewAESGCM([]byte("0123456789abcdef0123456789abcdef"))
	plaintext := []byte("same-data")

	c1, _ := c.Encrypt(plaintext)
	c2, _ := c.Encrypt(plaintext)
	assert.NotEqual(t, c1, c2, "两次加密结果应不同（nonce 不同）")
}

func TestAESGCM_InvalidKeyLength(t *testing.T) {
	_, err := NewAESGCM([]byte("short"))
	assert.Error(t, err)
}

func TestAESGCM_InvalidCiphertext(t *testing.T) {
	c, _ := NewAESGCM([]byte("0123456789abcdef0123456789abcdef"))
	_, err := c.Decrypt([]byte("too-short"))
	assert.Error(t, err)
}

func TestAESGCM_TamperedCiphertext(t *testing.T) {
	c, _ := NewAESGCM([]byte("0123456789abcdef0123456789abcdef"))

	ct, _ := c.Encrypt([]byte("important"))
	ct[len(ct)-1] ^= 0x01

	_, err := c.Decrypt(ct)
	assert.Error(t, err, "篡改密文应导致解密失败")
}

func TestAESGCM_FromHex(t *testing.T) {
	c, err := NewAESGCMFromHex("3031323334353637383961626364656630313233343536373839616263646566")
	require.NoError(t, err)

	enc, _ := c.EncryptString("test")
	dec, _ := c.DecryptString(enc)
	assert.Equal(t, "test", dec)
}

func TestAESGCM_FromPassword(t *testing.T) {
	c, err := NewAESGCMFromPassword("my-password", nil)
	require.NoError(t, err)

	ct, _ := c.EncryptString("hello")
	dt, _ := c.DecryptString(ct)
	assert.Equal(t, "hello", dt)
}

func TestRSAOAEP_GenerateEncryptDecrypt(t *testing.T) {
	privPEM, pubPEM, err := GenerateRSAKeyPair(2048)
	require.NoError(t, err)

	priv, err := NewRSAOAEPFromPrivateKey(privPEM)
	require.NoError(t, err)

	pub, err := NewRSAOAEPFromPublicKey(pubPEM)
	require.NoError(t, err)

	plaintext := []byte("RSA encrypted secret")
	ciphertext, err := pub.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := priv.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestRSAOAEP_String(t *testing.T) {
	privPEM, pubPEM, _ := GenerateRSAKeyPair(2048)
	priv, _ := NewRSAOAEPFromPrivateKey(privPEM)
	pub, _ := NewRSAOAEPFromPublicKey(pubPEM)

	enc, err := pub.EncryptString("hello rsa")
	require.NoError(t, err)

	dec, err := priv.DecryptString(enc)
	require.NoError(t, err)
	assert.Equal(t, "hello rsa", dec)
}

func TestGzip_CompressDecompress(t *testing.T) {
	c := NewGzipCompression(gzip.DefaultCompression)
	data := []byte("compressible data compressible data compressible data")

	compressed, err := c.Compress(data)
	require.NoError(t, err)
	assert.True(t, len(compressed) < len(data), "压缩后应更小")

	decompressed, err := c.Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, data, decompressed)
}

func TestGzip_EmptyData(t *testing.T) {
	c := NewGzipCompression(gzip.BestCompression)

	compressed, err := c.Compress([]byte{})
	require.NoError(t, err)

	decompressed, err := c.Decompress(compressed)
	require.NoError(t, err)
	assert.Empty(t, decompressed)
}

func TestZlib_CompressDecompress(t *testing.T) {
	c := NewZlibCompression(zlib.DefaultCompression)
	data := []byte("zlib compressible data zlib compressible data")

	compressed, err := c.Compress(data)
	require.NoError(t, err)
	assert.True(t, len(compressed) < len(data))

	decompressed, err := c.Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, data, decompressed)
}

func TestEncryptCompressChain(t *testing.T) {
	aes, _ := NewAESGCM([]byte("0123456789abcdef0123456789abcdef"))
	gz := NewGzipCompression(gzip.DefaultCompression)

	original := []byte("chain test data chain test data chain test data")
	encrypted, err := EncryptCompress(aes, gz, original)
	require.NoError(t, err)

	decrypted, err := DecryptDecompress(aes, gz, encrypted)
	require.NoError(t, err)
	assert.Equal(t, original, decrypted)
}

func TestGenerateSelfSignedCert(t *testing.T) {
	certPEM, keyPEM, err := GenerateSelfSignedCert("TestOrg", 2048)
	require.NoError(t, err)
	assert.Contains(t, certPEM, "BEGIN CERTIFICATE")
	assert.Contains(t, keyPEM, "BEGIN RSA PRIVATE KEY")
}

func TestHexDecode(t *testing.T) {
	data, err := hexDecode("0123456789abcdef")
	require.NoError(t, err)
	assert.Equal(t, 8, len(data))
}

func TestNewKeyManager_Versioning(t *testing.T) {
	km, err := NewKeyManager("0123456789abcdef0123456789abcdef", nil)
	require.NoError(t, err)
	assert.Equal(t, 1, km.CurrentVersion())

	enc, err := km.EncryptString("hello version")
	require.NoError(t, err)
	assert.Contains(t, enc, "v1:")

	dec, err := km.DecryptString(enc)
	require.NoError(t, err)
	assert.Equal(t, "hello version", dec)
}

func TestNewKeyManager_WithOldKeys(t *testing.T) {
	oldKeys := map[int]string{
		1: "00000000000000000000000000000000",
	}
	km, err := NewKeyManager("0123456789abcdef0123456789abcdef", oldKeys)
	require.NoError(t, err)
	assert.Equal(t, 2, km.CurrentVersion())

	oldKM, _ := NewKeyManager("00000000000000000000000000000000", nil)
	oldEnc, _ := oldKM.EncryptString("encrypted-with-old-key")
	assert.Contains(t, oldEnc, "v1:")

	dec, err := km.DecryptString(oldEnc)
	require.NoError(t, err)
	assert.Equal(t, "encrypted-with-old-key", dec)
}

func TestNewKeyManager_BackwardCompat_NoPrefix(t *testing.T) {
	km, _ := NewKeyManager("0123456789abcdef0123456789abcdef", nil)
	aes, _ := NewAESGCMFromHex("0123456789abcdef0123456789abcdef")

	oldEnc, _ := aes.EncryptString("legacy-data")

	dec, err := km.DecryptString(oldEnc)
	require.NoError(t, err)
	assert.Equal(t, "legacy-data", dec)
}

func TestKeyManager_KeyForVersion(t *testing.T) {
	km, _ := NewKeyManager("0123456789abcdef0123456789abcdef", nil)

	v1, err := km.KeyForVersion(1)
	require.NoError(t, err)

	_, err = km.KeyForVersion(999)
	assert.Error(t, err)
	_ = v1
}

type testUser struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email" encrypt:"true"`
	Phone    string `json:"phone" encrypt:"true"`
	Bio      string `json:"bio"`
}

type testUserPtr struct {
	ID       uint    `json:"id"`
	Email    *string `json:"email" encrypt:"true"`
	Nickname string  `json:"nickname"`
}

func TestFieldEncryptor_EncryptDecryptStruct(t *testing.T) {
	km, _ := NewKeyManager("0123456789abcdef0123456789abcdef", nil)
	fe := NewFieldEncryptor(km)

	user := testUser{
		ID:       1,
		Username: "alice",
		Email:    "alice@example.com",
		Phone:    "13800138000",
		Bio:      "hello world",
	}

	err := fe.EncryptFields(&user)
	require.NoError(t, err)
	assert.NotEqual(t, "alice@example.com", user.Email)
	assert.NotEqual(t, "13800138000", user.Phone)
	assert.Equal(t, "hello world", user.Bio, "无 tag 字段不应加密")
	assert.Contains(t, user.Email, "v1:")
	assert.Contains(t, user.Phone, "v1:")

	err = fe.DecryptFields(&user)
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", user.Email)
	assert.Equal(t, "13800138000", user.Phone)
}

func TestFieldEncryptor_EncryptDecryptEmpty(t *testing.T) {
	km, _ := NewKeyManager("0123456789abcdef0123456789abcdef", nil)
	fe := NewFieldEncryptor(km)

	user := testUser{ID: 2, Username: "bob", Email: "", Phone: ""}
	err := fe.EncryptFields(&user)
	require.NoError(t, err)
	assert.Equal(t, "", user.Email, "空字符串不应加密")
}

func TestFieldEncryptor_PtrFields(t *testing.T) {
	km, _ := NewKeyManager("0123456789abcdef0123456789abcdef", nil)
	fe := NewFieldEncryptor(km)

	email := "ptr@test.com"
	user := testUserPtr{ID: 3, Email: &email, Nickname: "ptr-user"}

	err := fe.EncryptFields(&user)
	require.NoError(t, err)
	assert.NotEqual(t, "ptr@test.com", *user.Email)

	err = fe.DecryptFields(&user)
	require.NoError(t, err)
	assert.Equal(t, "ptr@test.com", *user.Email)
}

func TestFieldEncryptor_Slice(t *testing.T) {
	km, _ := NewKeyManager("0123456789abcdef0123456789abcdef", nil)
	fe := NewFieldEncryptor(km)

	users := []testUser{
		{ID: 1, Email: "a@test.com", Phone: "111"},
		{ID: 2, Email: "b@test.com", Phone: "222"},
	}

	err := fe.EncryptSlice(&users)
	require.NoError(t, err)
	assert.NotEqual(t, "a@test.com", users[0].Email)
	assert.NotEqual(t, "b@test.com", users[1].Email)

	err = fe.DecryptSlice(&users)
	require.NoError(t, err)
	assert.Equal(t, "a@test.com", users[0].Email)
	assert.Equal(t, "b@test.com", users[1].Email)
}

func TestZipCompressDecompress(t *testing.T) {
	c := NewZipCompression("")
	data := []byte("zip compressed data for testing roundtrip")
	compressed, err := c.Compress(data)
	require.NoError(t, err)

	decompressed, err := c.Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, data, decompressed)
}

func TestZipWithPassword(t *testing.T) {
	c := NewZipCompression("mypassword")
	data := []byte("secret zip content with password protection")
	compressed, err := c.Compress(data)
	require.NoError(t, err)

	_, err = NewZipCompression("wrongpass").Decompress(compressed)
	assert.Error(t, err, "错误密码应解密失败")

	decompressed, err := NewZipCompression("mypassword").Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, data, decompressed)
}

func TestSplitCompression_Basic(t *testing.T) {
	c := NewSplitCompression(50, "", "part")
	data := []byte("this data will be split into multiple shards for testing")
	set, err := c.Compress(data)
	require.NoError(t, err)
	assert.Greater(t, len(set.Meta.ShardIDs), 1)

	recovered, err := c.Decompress(set)
	require.NoError(t, err)
	assert.Equal(t, data, recovered)
}

func TestSplitCompression_WithPassword(t *testing.T) {
	c := NewSplitCompression(30, "split-password", "shard")
	data := []byte("split with password protection across shards")
	set, err := c.Compress(data)
	require.NoError(t, err)

	recovered, err := c.Decompress(set)
	require.NoError(t, err)
	assert.Equal(t, data, recovered)
}

func TestSplitCompression_Serialize(t *testing.T) {
	c := NewSplitCompression(100, "", "seg")
	data := []byte("serialize and deserialize shard set test data")
	set, _ := c.Compress(data)

	serialized, err := c.Serialize(set)
	require.NoError(t, err)

	deserialized, err := c.Deserialize(serialized)
	require.NoError(t, err)
	assert.Equal(t, set.Meta.TotalSize, deserialized.Meta.TotalSize)
	assert.Equal(t, len(set.Meta.ShardIDs), len(deserialized.Meta.ShardIDs))

	recovered, err := c.Decompress(deserialized)
	require.NoError(t, err)
	assert.Equal(t, data, recovered)
}

func TestSplitCompression_MissingShard(t *testing.T) {
	c := NewSplitCompression(20, "", "x")
	data := []byte("missing shard test data")
	set, _ := c.Compress(data)
	delete(set.Shards, set.Meta.ShardIDs[0])

	_, err := c.Decompress(set)
	assert.Error(t, err)
}

func TestDetectCompression(t *testing.T) {
	gz := NewGzipCompression(gzip.DefaultCompression)
	gzData, _ := gz.Compress([]byte("test"))
	assert.Equal(t, "gzip", DetectCompression(gzData))

	z := NewZlibCompression(zlib.DefaultCompression)
	zData, _ := z.Compress([]byte("test"))
	assert.Equal(t, "zlib", DetectCompression(zData))

	zp := NewZipCompression("")
	zpData, _ := zp.Compress([]byte("test"))
	assert.Equal(t, "zip", DetectCompression(zpData))

	assert.Equal(t, "unknown", DetectCompression([]byte{0, 0, 0}))
}

func TestCompressAutoDecompressAuto(t *testing.T) {
	data := []byte("auto detect compression test data")
	for _, method := range []string{"gzip", "zlib", "zip"} {
		compressed, err := CompressAuto(data, method, gzip.DefaultCompression, "")
		require.NoError(t, err, "method=%s", method)

		decompressed, err := DecompressAuto(compressed, "")
		require.NoError(t, err, "method=%s", method)
		assert.Equal(t, data, decompressed, "method=%s", method)
	}
}

func TestIsEncrypted(t *testing.T) {
	assert.True(t, isEncrypted("v1:abcdef123456"))
	assert.True(t, isEncrypted("abcdef1234567890abcdef1234567890abcdef12"))
	assert.False(t, isEncrypted(""))
	assert.False(t, isEncrypted("plain"))
}
