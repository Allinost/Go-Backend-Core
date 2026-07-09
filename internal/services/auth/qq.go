package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type QQProvider struct {
	appID      string
	appKey     string
	redirect   string
	httpClient *http.Client
}

func NewQQProvider(appID, appKey, redirect string) *QQProvider {
	return &QQProvider{
		appID:      appID,
		appKey:     appKey,
		redirect:   redirect,
		httpClient: http.DefaultClient,
	}
}

func (p *QQProvider) Name() string {
	return "qq"
}

func (p *QQProvider) AuthURL(state string) string {
	return fmt.Sprintf(
		"https://graph.qq.com/oauth2.0/authorize?response_type=code&client_id=%s&redirect_uri=%s&state=%s&scope=get_user_info",
		p.appID, url.QueryEscape(p.redirect), state,
	)
}

func (p *QQProvider) Exchange(ctx context.Context, code string) (string, error) {
	u := fmt.Sprintf(
		"https://graph.qq.com/oauth2.0/token?grant_type=authorization_code&client_id=%s&client_secret=%s&code=%s&redirect_uri=%s&fmt=json",
		p.appID, p.appKey, code, url.QueryEscape(p.redirect),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("qq: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		AccessToken string `json:"access_token"`
		Error       int    `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("qq: 解析响应失败: %w", err)
	}
	if result.Error != 0 {
		return "", fmt.Errorf("qq: %s", result.ErrorDesc)
	}
	return result.AccessToken, nil
}

func (p *QQProvider) GetUserInfo(ctx context.Context, accessToken string) (*SocialUser, error) {
	openID, err := p.getOpenID(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	u := fmt.Sprintf(
		"https://graph.qq.com/user/get_user_info?access_token=%s&oauth_consumer_key=%s&openid=%s",
		accessToken, p.appID, openID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qq: 获取用户信息失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Ret      int    `json:"ret"`
		Msg      string `json:"msg"`
		Nickname string `json:"nickname"`
		Avatar   string `json:"figureurl_qq_2"`
		Gender   string `json:"gender"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("qq: 解析用户信息失败: %w", err)
	}
	if result.Ret != 0 {
		return nil, fmt.Errorf("qq: %s", result.Msg)
	}

	gender := 0
	if result.Gender == "男" {
		gender = 1
	} else if result.Gender == "女" {
		gender = 2
	}

	return &SocialUser{
		OpenID:    openID,
		Nickname:  result.Nickname,
		AvatarURL: result.Avatar,
		Gender:    gender,
	}, nil
}

func (p *QQProvider) getOpenID(ctx context.Context, accessToken string) (string, error) {
	u := fmt.Sprintf(
		"https://graph.qq.com/oauth2.0/me?access_token=%s&fmt=json",
		accessToken,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("qq: 获取 OpenID 失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		OpenID  string `json:"openid"`
		UnionID string `json:"unionid"`
		Error   int    `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("qq: 解析 OpenID 失败: %w", err)
	}
	if result.Error != 0 {
		return "", fmt.Errorf("qq: 获取 OpenID 错误: %d", result.Error)
	}
	return result.OpenID, nil
}
