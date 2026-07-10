package crypto

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Compression 定义数据压缩/解压接口，支持多种压缩算法实现
type Compression interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
}

// GzipCompression 使用 gzip 算法进行数据压缩/解压
type GzipCompression struct {
	level int
}

// NewGzipCompression 创建 gzip 压缩器，level 超出范围时使用默认级别
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

// Decompress 解压 gzip 格式的数据
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

// ZlibCompression 使用 zlib 算法进行数据压缩/解压
type ZlibCompression struct {
	level int
}

// NewZlibCompression 创建 zlib 压缩器，level 超出范围时使用默认级别
func NewZlibCompression(level int) *ZlibCompression {
	if level < zlib.HuffmanOnly || level > zlib.BestCompression {
		level = zlib.DefaultCompression
	}
	return &ZlibCompression{level: level}
}

// Compress 使用 zlib 压缩数据
func (c *ZlibCompression) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := zlib.NewWriterLevel(&buf, c.level)
	if err != nil {
		return nil, fmt.Errorf("crypto: zlib writer 初始化失败: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("crypto: zlib 压缩失败: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("crypto: zlib 关闭失败: %w", err)
	}
	return buf.Bytes(), nil
}

// Decompress 解压 zlib 格式的数据
func (c *ZlibCompression) Decompress(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("crypto: zlib reader 初始化失败: %w", err)
	}
	defer r.Close()
	result, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("crypto: zlib 解压失败: %w", err)
	}
	return result, nil
}

// ZipCompression 使用 zip 格式进行数据压缩/解压，可选密码加密
type ZipCompression struct {
	password string
}

// NewZipCompression 创建 zip 压缩器，password 非空时对 zip 数据额外加密
func NewZipCompression(password string) *ZipCompression {
	return &ZipCompression{password: password}
}

// Compress 将数据打包为 zip 格式，若设置了密码则对 zip 数据加密
func (c *ZipCompression) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	f, err := w.Create("data")
	if err != nil {
		return nil, fmt.Errorf("crypto: zip 创建条目失败: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		return nil, fmt.Errorf("crypto: zip 写入失败: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("crypto: zip 关闭失败: %w", err)
	}

	result := buf.Bytes()
	if c.password != "" {
		enc, err := encryptBytes(result, c.password)
		if err != nil {
			return nil, err
		}
		return enc, nil
	}
	return result, nil
}

// Decompress 解压 zip 数据，若设置了密码则先解密再解压
func (c *ZipCompression) Decompress(data []byte) ([]byte, error) {
	raw := data
	if c.password != "" {
		dec, err := decryptBytes(data, c.password)
		if err != nil {
			return nil, fmt.Errorf("crypto: zip 密码解密失败: %w", err)
		}
		raw = dec
	}

	r, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("crypto: zip reader 初始化失败: %w", err)
	}

	var result []byte
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("crypto: zip 打开条目 %s 失败: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("crypto: zip 读取条目 %s 失败: %w", f.Name, err)
		}
		result = append(result, data...)
	}
	return result, nil
}

// ShardMeta 分片压缩的元数据，记录版本、总大小、分片大小、分片ID列表及是否有密码
type ShardMeta struct {
	Version   int      `json:"version"`            // 分片格式版本号
	TotalSize int64    `json:"total_size"`         // 加密后数据总大小
	ShardSize int64    `json:"shard_size"`         // 每个分片的大小
	ShardIDs  []string `json:"shard_ids"`          // 分片 ID 有序列表
	Password  bool     `json:"password,omitempty"` // 是否使用密码加密
}

// SplitCompression 将数据分片压缩/加密，支持按大小切分为多个分片
type SplitCompression struct {
	shardSize int64
	password  string
	prefix    string
}

// NewSplitCompression 创建分片压缩器，shardSize 默认为 10MB，prefix 默认为 "shard"
func NewSplitCompression(shardSize int64, password, prefix string) *SplitCompression {
	if shardSize <= 0 {
		shardSize = 10 * 1024 * 1024 // 10MB default
	}
	if prefix == "" {
		prefix = "shard"
	}
	return &SplitCompression{
		shardSize: shardSize,
		password:  password,
		prefix:    prefix,
	}
}

// ShardSet 包含分片元数据及所有分片数据
type ShardSet struct {
	Meta   ShardMeta
	Shards map[string][]byte
}

// Compress 将数据加密（若设置密码）后按 shardSize 切分为多个分片
func (c *SplitCompression) Compress(data []byte) (*ShardSet, error) {
	encrypted := data
	if c.password != "" {
		var err error
		encrypted, err = encryptBytes(data, c.password)
		if err != nil {
			return nil, err
		}
	}

	total := int64(len(encrypted))
	numShards := (total + c.shardSize - 1) / c.shardSize
	shards := make(map[string][]byte)

	for i := int64(0); i < numShards; i++ {
		start := i * c.shardSize
		end := start + c.shardSize
		if end > total {
			end = total
		}
		shardID := fmt.Sprintf("%s_%03d", c.prefix, i)
		shards[shardID] = encrypted[start:end]
	}

	var ids []string
	for i := int64(0); i < numShards; i++ {
		ids = append(ids, fmt.Sprintf("%s_%03d", c.prefix, i))
	}

	meta := ShardMeta{
		Version:   1,
		TotalSize: total,
		ShardSize: c.shardSize,
		ShardIDs:  ids,
		Password:  c.password != "",
	}

	return &ShardSet{Meta: meta, Shards: shards}, nil
}

// Decompress 按分片 ID 顺序合并数据，若设置密码则解密后返回
func (c *SplitCompression) Decompress(set *ShardSet) ([]byte, error) {
	if len(set.Meta.ShardIDs) == 0 {
		return nil, fmt.Errorf("crypto: 分片清单为空")
	}

	var buf bytes.Buffer
	for _, id := range set.Meta.ShardIDs {
		shard, ok := set.Shards[id]
		if !ok {
			return nil, fmt.Errorf("crypto: 缺少分片 %s", id)
		}
		buf.Write(shard)
	}

	encrypted := buf.Bytes()
	if c.password != "" {
		return decryptBytes(encrypted, c.password)
	}
	return encrypted, nil
}

// Serialize 将 ShardSet 序列化为 JSON 格式
func (c *SplitCompression) Serialize(set *ShardSet) ([]byte, error) {
	metaJSON, _ := json.Marshal(set.Meta)

	shardsB64 := make(map[string]string)
	for k, v := range set.Shards {
		shardsB64[k] = string(v)
	}

	container := map[string]any{
		"meta":   string(metaJSON),
		"shards": shardsB64,
	}
	return json.Marshal(container)
}

// Deserialize 从 JSON 数据反序列化为 ShardSet
func (c *SplitCompression) Deserialize(data []byte) (*ShardSet, error) {
	var container struct {
		Meta   string            `json:"meta"`
		Shards map[string]string `json:"shards"`
	}
	if err := json.Unmarshal(data, &container); err != nil {
		return nil, fmt.Errorf("crypto: 反序列化分片集失败: %w", err)
	}

	var meta ShardMeta
	if err := json.Unmarshal([]byte(container.Meta), &meta); err != nil {
		return nil, fmt.Errorf("crypto: 解析清单失败: %w", err)
	}

	shards := make(map[string][]byte)
	for k, v := range container.Shards {
		shards[k] = []byte(v)
	}

	return &ShardSet{Meta: meta, Shards: shards}, nil
}

func encryptBytes(data []byte, password string) ([]byte, error) {
	key := deriveKey(password, fixedSalt, 32)
	aes, err := NewAESGCM(key)
	if err != nil {
		return nil, err
	}
	return aes.Encrypt(data)
}

func decryptBytes(data []byte, password string) ([]byte, error) {
	key := deriveKey(password, fixedSalt, 32)
	aes, err := NewAESGCM(key)
	if err != nil {
		return nil, err
	}
	return aes.Decrypt(data)
}

var fixedSalt = []byte("GBk_CrypT0_Z!p_S@lt_2024")

// EncryptCompress 先压缩再加密，组合压缩与加密操作
func EncryptCompress(cipher Cipher, comp Compression, plaintext []byte) ([]byte, error) {
	compressed, err := comp.Compress(plaintext)
	if err != nil {
		return nil, err
	}
	return cipher.Encrypt(compressed)
}

// DecryptDecompress 先解密再解压，组合解密与解压操作
func DecryptDecompress(cipher Cipher, comp Compression, ciphertext []byte) ([]byte, error) {
	decrypted, err := cipher.Decrypt(ciphertext)
	if err != nil {
		return nil, err
	}
	return comp.Decompress(decrypted)
}

// DetectCompression 通过文件头魔数检测数据使用的压缩格式 (gzip/zlib/zip/shard)
func DetectCompression(data []byte) string {
	if len(data) < 2 {
		return "unknown"
	}
	if data[0] == 0x1f && data[1] == 0x8b {
		return "gzip"
	}
	if data[0] == 0x78 {
		return "zlib"
	}
	if len(data) > 4 && data[0] == 0x50 && data[1] == 0x4B && data[2] == 0x03 && data[3] == 0x04 {
		return "zip"
	}
	if len(data) > 2 && data[0] == '{' {
		return "shard"
	}
	return "unknown"
}

// CompressAuto 根据指定的 method 自动选择压缩算法并压缩数据
func CompressAuto(data []byte, method string, level int, password string) ([]byte, error) {
	switch strings.ToLower(method) {
	case "gzip":
		c := NewGzipCompression(level)
		return c.Compress(data)
	case "zlib":
		c := NewZlibCompression(level)
		return c.Compress(data)
	case "zip":
		c := NewZipCompression(password)
		return c.Compress(data)
	default:
		return nil, fmt.Errorf("crypto: 不支持的压缩方式 %s", method)
	}
}

// DecompressAuto 自动检测数据压缩格式并选用对应算法解压
func DecompressAuto(data []byte, password string) ([]byte, error) {
	method := DetectCompression(data)
	switch method {
	case "gzip":
		c := NewGzipCompression(gzip.DefaultCompression)
		return c.Decompress(data)
	case "zlib":
		c := NewZlibCompression(zlib.DefaultCompression)
		return c.Decompress(data)
	case "zip":
		c := NewZipCompression(password)
		return c.Decompress(data)
	default:
		return nil, fmt.Errorf("crypto: 无法检测压缩格式")
	}
}
