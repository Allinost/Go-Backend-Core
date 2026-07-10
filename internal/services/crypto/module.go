package crypto

import (
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Allinost/go-backend-core/internal/config"
)

type Module struct {
	name string
	cfg  config.CryptoConfig
	km   *KeyManager
}

func NewModule() *Module {
	return &Module{name: "crypto"}
}

func (m *Module) Name() string { return m.name }

func (m *Module) Init(cfg *config.Config) error {
	m.cfg = cfg.Crypto
	if !m.cfg.Enabled {
		return nil
	}
	if m.cfg.MasterKeyHex != "" {
		km, err := NewKeyManager(m.cfg.MasterKeyHex, m.cfg.OldKeys)
		if err != nil {
			return err
		}
		m.km = km
	}
	return nil
}

func (m *Module) Close() error { return nil }

func (m *Module) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/encrypt", m.handleEncrypt)
	rg.POST("/decrypt", m.handleDecrypt)
	rg.POST("/compress", m.handleCompress)
	rg.POST("/decompress", m.handleDecompress)
	rg.POST("/hash", m.handleHash)
}

type encryptReq struct {
	Data   string `json:"data" binding:"required"`
	KeyHex string `json:"key_hex,omitempty"`
	Algo   string `json:"algo"`
}

type cryptoResp struct {
	Result string `json:"result"`
	Algo   string `json:"algo"`
}

// handleEncrypt 数据加密
// @Summary      数据加密
// @Description  使用指定算法（aes-gcm / chacha20-poly1305）加密数据
// @Tags         crypto
// @Accept       json
// @Produce      json
// @Param        body  body  encryptReq  true  "加密请求"
// @Success      200   {object}  cryptoResp
// @Failure      400   {object}  object{error=string}
// @Failure      500   {object}  object{error=string}
// @Router       /crypto/encrypt [post]
func (m *Module) handleEncrypt(c *gin.Context) {
	var req encryptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	algo := req.Algo
	if algo == "" {
		algo = "aes-gcm"
	}

	var out string
	switch algo {
	case "aes-gcm", "":
		if m.km != nil {
			enc, err := m.km.EncryptString(req.Data)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			out = enc
		} else if req.KeyHex != "" {
			aes, err := NewAESGCMFromHex(req.KeyHex)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			enc, err := aes.EncryptString(req.Data)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			out = enc
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "需要 key_hex 或配置 master_key_hex"})
			return
		}
	case "chacha20-poly1305":
		if req.KeyHex == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "chacha20 需要 key_hex 参数"})
			return
		}
		chacha, err := NewChaCha20Poly1305FromHex(req.KeyHex)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		enc, err := chacha.EncryptString(req.Data)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		out = enc
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的加密算法: " + algo})
		return
	}

	c.JSON(http.StatusOK, cryptoResp{Result: out, Algo: algo})
}

type decryptReq struct {
	Data   string `json:"data" binding:"required"`
	KeyHex string `json:"key_hex,omitempty"`
	Algo   string `json:"algo"`
}

// handleDecrypt 数据解密
// @Summary      数据解密
// @Description  使用指定算法解密已加密的数据
// @Tags         crypto
// @Accept       json
// @Produce      json
// @Param        body  body  decryptReq  true  "解密请求"
// @Success      200   {object}  cryptoResp
// @Failure      400   {object}  object{error=string}
// @Failure      500   {object}  object{error=string}
// @Router       /crypto/decrypt [post]
func (m *Module) handleDecrypt(c *gin.Context) {
	var req decryptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	algo := req.Algo
	if algo == "" {
		algo = "aes-gcm"
	}

	var out string
	switch algo {
	case "aes-gcm", "":
		if m.km != nil {
			dec, err := m.km.DecryptString(req.Data)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			out = dec
		} else if req.KeyHex != "" {
			aes, err := NewAESGCMFromHex(req.KeyHex)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			dec, err := aes.DecryptString(req.Data)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			out = dec
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "需要 key_hex 或配置 master_key_hex"})
			return
		}
	case "chacha20-poly1305":
		if req.KeyHex == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "chacha20 需要 key_hex 参数"})
			return
		}
		chacha, err := NewChaCha20Poly1305FromHex(req.KeyHex)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		dec, err := chacha.DecryptString(req.Data)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		out = dec
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的加密算法: " + algo})
		return
	}

	c.JSON(http.StatusOK, cryptoResp{Result: out, Algo: algo})
}

type compressReq struct {
	Data     string `json:"data" binding:"required"`
	Method   string `json:"method"`
	Level    int    `json:"level"`
	Password string `json:"password,omitempty"`
}

type compressResp struct {
	Result string `json:"result"`
	Method string `json:"method"`
	Raw    string `json:"raw,omitempty"`
}

// handleCompress 数据压缩
// @Summary      数据压缩
// @Description  使用 gzip/zlib/zstd/zip 算法压缩数据，结果返回 hex 编码
// @Tags         crypto
// @Accept       json
// @Produce      json
// @Param        body  body  compressReq  true  "压缩请求"
// @Success      200   {object}  compressResp
// @Failure      400   {object}  object{error=string}
// @Failure      500   {object}  object{error=string}
// @Router       /crypto/compress [post]
func (m *Module) handleCompress(c *gin.Context) {
	var req compressReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Method == "" {
		req.Method = "gzip"
	}

	compressed, err := CompressAuto([]byte(req.Data), req.Method, req.Level, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, compressResp{
		Result: hex.EncodeToString(compressed),
		Method: req.Method,
	})
}

type decompressReq struct {
	Data     string `json:"data" binding:"required"`
	Password string `json:"password,omitempty"`
}

// handleDecompress 数据解压
// @Summary      数据解压
// @Description  自动检测压缩算法并解压 hex 编码的压缩数据
// @Tags         crypto
// @Accept       json
// @Produce      json
// @Param        body  body  decompressReq  true  "解压请求"
// @Success      200   {object}  compressResp
// @Failure      400   {object}  object{error=string}
// @Failure      500   {object}  object{error=string}
// @Router       /crypto/decompress [post]
func (m *Module) handleDecompress(c *gin.Context) {
	var req decompressReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	raw, err := hex.DecodeString(req.Data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hex 解码失败: " + err.Error()})
		return
	}

	method := DetectCompression(raw)
	decompressed, err := DecompressAuto(raw, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, compressResp{
		Result: string(decompressed),
		Method: method,
	})
}

type hashReq struct {
	Data string `json:"data" binding:"required"`
	Algo string `json:"algo"`
}

// handleHash 哈希计算
// @Summary      哈希计算
// @Description  使用 sha256 / sha512 / blake3 算法计算数据哈希值
// @Tags         crypto
// @Accept       json
// @Produce      json
// @Param        body  body  hashReq  true  "哈希请求"
// @Success      200   {object}  cryptoResp
// @Failure      400   {object}  object{error=string}
// @Router       /crypto/hash [post]
func (m *Module) handleHash(c *gin.Context) {
	var req hashReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Algo == "" {
		req.Algo = "sha256"
	}

	var out string
	switch req.Algo {
	case "sha256":
		out = SHA256Hash([]byte(req.Data))
	case "sha512":
		out = SHA512Hash([]byte(req.Data))
	case "blake3":
		out = BLAKE3Hash([]byte(req.Data))
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的哈希算法: " + req.Algo})
		return
	}

	c.JSON(http.StatusOK, cryptoResp{Result: out, Algo: req.Algo})
}
