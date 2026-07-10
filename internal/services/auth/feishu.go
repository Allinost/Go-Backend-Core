package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// FeishuProvider 飞书第三方登录提供商
type FeishuProvider struct {
	appID      string
	appSecret  string
	redirect   string
	httpClient *http.Client
}

// NewFeishuProvider 创建飞书登录提供商实例
func NewFeishuProvider(appID, appSecret, redirect string) *FeishuProvider {
	return &FeishuProvider{
		appID:      appID,
		appSecret:  appSecret,
		redirect:   redirect,
		httpClient: http.DefaultClient,
	}
}

// Name 返回提供商名称
func (p *FeishuProvider) Name() string {
	return "feishu"
}

// AuthURL 生成飞书 OAuth 授权 URL
func (p *FeishuProvider) AuthURL(state string) string {
	return fmt.Sprintf(
		"https://open.feishu.cn/open-apis/authen/v1/index?app_id=%s&redirect_uri=%s&state=%s",
		p.appID, p.redirect, state,
	)
}

// Exchange 使用授权码交换飞书 access_token
func (p *FeishuProvider) Exchange(ctx context.Context, code string) (string, error) {
	body := map[string]string{
		"grant_type":   "authorization_code",
		"code":         code,
		"app_id":       p.appID,
		"app_secret":   p.appSecret,
		"redirect_uri": p.redirect,
	}
	payload, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://open.feishu.cn/open-apis/authen/v1/access_token",
		bytes.NewReader(payload),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("feishu: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("feishu: 解析响应失败: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("feishu: %s", result.Msg)
	}

	return result.Data.AccessToken, nil
}

// GetUserInfo 使用 access_token 获取飞书用户信息
func (p *FeishuProvider) GetUserInfo(ctx context.Context, accessToken string) (*SocialUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://open.feishu.cn/open-apis/authen/v1/user_info", nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("feishu: 获取用户信息失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			OpenID   string `json:"open_id"`
			UnionID  string `json:"union_id"`
			Nickname string `json:"name"`
			Avatar   string `json:"avatar_url"`
			Email    string `json:"email"`
			Phone    string `json:"mobile"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("feishu: 解析用户信息失败: %w", err)
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("feishu: %s", result.Msg)
	}

	return &SocialUser{
		OpenID:    result.Data.OpenID,
		UnionID:   result.Data.UnionID,
		Nickname:  result.Data.Nickname,
		AvatarURL: result.Data.Avatar,
		Email:     result.Data.Email,
		Phone:     result.Data.Phone,
	}, nil
}
