package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type AppleProvider struct {
	clientID   string
	teamID     string
	keyID      string
	privateKey string
	redirect   string
	httpClient *http.Client
}

func NewAppleProvider(clientID, teamID, keyID, privateKey, redirect string) *AppleProvider {
	return &AppleProvider{
		clientID:   clientID,
		teamID:     teamID,
		keyID:      keyID,
		privateKey: privateKey,
		redirect:   redirect,
		httpClient: http.DefaultClient,
	}
}

func (p *AppleProvider) Name() string {
	return "apple"
}

func (p *AppleProvider) AuthURL(state string) string {
	return fmt.Sprintf(
		"https://appleid.apple.com/auth/authorize?response_type=code&client_id=%s&redirect_uri=%s&state=%s&scope=name+email&response_mode=form_post",
		p.clientID, url.QueryEscape(p.redirect), state,
	)
}

func (p *AppleProvider) Exchange(ctx context.Context, code string) (string, error) {
	data := url.Values{
		"client_id":     {p.clientID},
		"client_secret": {p.clientSecret()},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {p.redirect},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://appleid.apple.com/auth/token",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("apple: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("apple: 解析响应失败: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("apple: %s", result.ErrorDesc)
	}
	return result.IDToken, nil
}

func (p *AppleProvider) GetUserInfo(ctx context.Context, idToken string) (*SocialUser, error) {
	claims, err := p.decodeIDToken(idToken)
	if err != nil {
		return nil, fmt.Errorf("apple: 解析 ID Token 失败: %w", err)
	}

	return &SocialUser{
		OpenID:   claims.Sub,
		Nickname: claims.DisplayName(),
		Email:    claims.Email,
	}, nil
}

type appleIDTokenClaims struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	IsPrivate     bool   `json:"is_private_email"`
	Name          struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
	} `json:"name"`
}

func (c *appleIDTokenClaims) DisplayName() string {
	if c.Name.FirstName != "" || c.Name.LastName != "" {
		return strings.TrimSpace(c.Name.FirstName + " " + c.Name.LastName)
	}
	return "AppleUser"
}

func (p *AppleProvider) decodeIDToken(tokenStr string) (*appleIDTokenClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("apple: 无效的 ID Token 格式")
	}

	payload := parts[1]
	switch l := len(payload) % 4; l {
	case 0:
	case 2:
		payload += "=="
	case 3:
		payload += "="
	default:
		return nil, fmt.Errorf("apple: base64 填充错误")
	}

	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, err
	}

	var claims appleIDTokenClaims
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil, fmt.Errorf("apple: 解析 payload 失败: %w", err)
	}
	return &claims, nil
}

func (p *AppleProvider) clientSecret() string {
	return p.privateKey
}
