package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// HonorProvider 荣耀第三方登录提供商
type HonorProvider struct {
	clientID     string
	clientSecret string
	redirect     string
	httpClient   *http.Client
}

// NewHonorProvider 创建荣耀登录提供商实例
func NewHonorProvider(clientID, clientSecret, redirect string) *HonorProvider {
	return &HonorProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirect:     redirect,
		httpClient:   http.DefaultClient,
	}
}

// Name 返回提供商名称
func (p *HonorProvider) Name() string {
	return "honor"
}

// AuthURL 生成荣耀 OAuth 授权 URL
func (p *HonorProvider) AuthURL(state string) string {
	return fmt.Sprintf(
		"https://login.honor.com/oauth2/v1/authorize?response_type=code&client_id=%s&redirect_uri=%s&state=%s&scope=openid+profile",
		p.clientID, url.QueryEscape(p.redirect), state,
	)
}

// Exchange 使用授权码交换荣耀 access_token
func (p *HonorProvider) Exchange(ctx context.Context, code string) (string, error) {
	body := map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"client_id":     p.clientID,
		"client_secret": p.clientSecret,
		"redirect_uri":  p.redirect,
	}
	payload, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://login.honor.com/oauth2/v1/token",
		bytes.NewReader(payload),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("honor: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("honor: 解析响应失败: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("honor: %s", result.ErrorDesc)
	}
	return result.AccessToken, nil
}

// GetUserInfo 使用 access_token 获取荣耀用户信息
func (p *HonorProvider) GetUserInfo(ctx context.Context, accessToken string) (*SocialUser, error) {
	u := fmt.Sprintf(
		"https://api.honor.com/rest/user/v1/userinfo?access_token=%s",
		accessToken,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("honor: 获取用户信息失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		OpenID   string `json:"openID"`
		Nickname string `json:"nickName"`
		Avatar   string `json:"avatar"`
		Gender   string `json:"gender"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("honor: 解析用户信息失败: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("honor: %s", result.Error)
	}

	gender := 0
	switch result.Gender {
	case "1":
		gender = 1
	case "2":
		gender = 2
	}

	return &SocialUser{
		OpenID:    result.OpenID,
		Nickname:  result.Nickname,
		AvatarURL: result.Avatar,
		Gender:    gender,
	}, nil
}
