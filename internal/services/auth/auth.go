package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenType JWT token 类型
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"  // 访问令牌
	TokenTypeRefresh TokenType = "refresh" // 刷新令牌
)

// Claims JWT 自定义声明，包含用户身份信息和 token 类型
type Claims struct {
	UserID    uint      `json:"user_id"`
	Username  string    `json:"username"`
	TokenType TokenType `json:"token_type"`
	jwt.RegisteredClaims
}

// Service JWT token 签发与验证服务
type Service struct {
	secret        []byte
	accessExpire  time.Duration
	refreshExpire time.Duration
	issuer        string
}

// Config JWT 服务配置
type Config struct {
	Secret        string
	AccessExpire  time.Duration
	RefreshExpire time.Duration
	Issuer        string
}

// TokenPair 访问令牌与刷新令牌对
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	UserID       uint   `json:"user_id"`
	Username     string `json:"username"`
}

// NewService 创建 JWT 服务实例，设置默认值
func NewService(cfg Config) *Service {
	if cfg.Issuer == "" {
		cfg.Issuer = "go-backend-core"
	}
	if cfg.AccessExpire <= 0 {
		cfg.AccessExpire = 30 * time.Minute
	}
	if cfg.RefreshExpire <= 0 {
		cfg.RefreshExpire = 7 * 24 * time.Hour
	}
	return &Service{
		secret:        []byte(cfg.Secret),
		accessExpire:  cfg.AccessExpire,
		refreshExpire: cfg.RefreshExpire,
		issuer:        cfg.Issuer,
	}
}

// GenerateTokenPair 生成访问令牌和刷新令牌对
func (s *Service) GenerateTokenPair(userID uint, username string) (*TokenPair, error) {
	now := time.Now()

	accessToken, err := s.generateToken(userID, username, TokenTypeAccess, s.accessExpire, now)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.generateToken(userID, username, TokenTypeRefresh, s.refreshExpire, now)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.accessExpire.Seconds()),
		UserID:       userID,
		Username:     username,
	}, nil
}

func (s *Service) generateToken(userID uint, username string, tokenType TokenType, expire time.Duration, now time.Time) (string, error) {
	claims := Claims{
		UserID:    userID,
		Username:  username,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expire)),
			Subject:   fmt.Sprintf("%d", userID),
			ID:        fmt.Sprintf("%s-%d-%d", tokenType, userID, now.UnixNano()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ValidateToken 验证 token 并返回声明（不检查 token 类型）
func (s *Service) ValidateToken(tokenStr string) (*Claims, error) {
	return s.parseToken(tokenStr, "")
}

// ValidateAccessToken 验证访问令牌
func (s *Service) ValidateAccessToken(tokenStr string) (*Claims, error) {
	return s.parseToken(tokenStr, TokenTypeAccess)
}

// ValidateRefreshToken 验证刷新令牌
func (s *Service) ValidateRefreshToken(tokenStr string) (*Claims, error) {
	return s.parseToken(tokenStr, TokenTypeRefresh)
}

func (s *Service) parseToken(tokenStr string, expectedType TokenType) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: 非预期的签名算法 %v", token.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("auth: 无效的 token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("auth: token 验证失败")
	}

	if expectedType != "" && claims.TokenType != expectedType {
		return nil, fmt.Errorf("auth: token 类型不匹配，期望 %s，实际 %s", expectedType, claims.TokenType)
	}

	return claims, nil
}

// RefreshAccessToken 使用刷新令牌生成新的 token 对
func (s *Service) RefreshAccessToken(refreshTokenStr string) (*TokenPair, error) {
	claims, err := s.ValidateRefreshToken(refreshTokenStr)
	if err != nil {
		return nil, err
	}

	return s.GenerateTokenPair(claims.UserID, claims.Username)
}
