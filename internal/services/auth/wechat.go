package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// WechatProvider 微信第三方登录提供商
type WechatProvider struct {
	appID      string
	appSecret  string
	redirect   string
	httpClient *http.Client
}

// NewWechatProvider 创建微信登录提供商实例
func NewWechatProvider(appID, appSecret, redirect string) *WechatProvider {
	return &WechatProvider{
		appID:      appID,
		appSecret:  appSecret,
		redirect:   redirect,
		httpClient: http.DefaultClient,
	}
}

// Name 返回提供商名称
func (p *WechatProvider) Name() string {
	return "wechat"
}

// AuthURL 生成微信扫码登录授权 URL
func (p *WechatProvider) AuthURL(state string) string {
	return fmt.Sprintf(
		"https://open.weixin.qq.com/connect/qrconnect?appid=%s&redirect_uri=%s&response_type=code&scope=snsapi_login&state=%s#wechat_redirect",
		p.appID, url.QueryEscape(p.redirect), state,
	)
}

// Exchange 使用授权码交换微信 access_token
func (p *WechatProvider) Exchange(ctx context.Context, code string) (string, error) {
	u := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/oauth2/access_token?appid=%s&secret=%s&code=%s&grant_type=authorization_code",
		p.appID, p.appSecret, code,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("wechat: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		AccessToken string `json:"access_token"`
		OpenID      string `json:"openid"`
		UnionID     string `json:"unionid"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("wechat: 解析响应失败: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("wechat: %s", result.ErrMsg)
	}

	return result.AccessToken, nil
}

// GetUserInfo 使用 access_token 获取微信用户信息
func (p *WechatProvider) GetUserInfo(ctx context.Context, accessToken string) (*SocialUser, error) {
	u := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/userinfo?access_token=%s&openid=%s",
		accessToken, "",
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wechat: 获取用户信息失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		OpenID   string `json:"openid"`
		UnionID  string `json:"unionid"`
		Nickname string `json:"nickname"`
		Avatar   string `json:"headimgurl"`
		Gender   int    `json:"sex"`
		ErrCode  int    `json:"errcode"`
		ErrMsg   string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("wechat: 解析用户信息失败: %w", err)
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("wechat: %s", result.ErrMsg)
	}

	return &SocialUser{
		OpenID:    result.OpenID,
		UnionID:   result.UnionID,
		Nickname:  result.Nickname,
		AvatarURL: result.Avatar,
		Gender:    result.Gender,
	}, nil
}
