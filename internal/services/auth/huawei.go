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

type HuaweiProvider struct {
	clientID     string
	clientSecret string
	redirect     string
	httpClient   *http.Client
}

func NewHuaweiProvider(clientID, clientSecret, redirect string) *HuaweiProvider {
	return &HuaweiProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirect:     redirect,
		httpClient:   http.DefaultClient,
	}
}

func (p *HuaweiProvider) Name() string {
	return "huawei"
}

func (p *HuaweiProvider) AuthURL(state string) string {
	return fmt.Sprintf(
		"https://oauth-login.cloud.huawei.com/oauth2/v3/authorize?response_type=code&client_id=%s&redirect_uri=%s&state=%s&scope=openid+profile",
		p.clientID, url.QueryEscape(p.redirect), state,
	)
}

func (p *HuaweiProvider) Exchange(ctx context.Context, code string) (string, error) {
	body := map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"client_id":     p.clientID,
		"client_secret": p.clientSecret,
		"redirect_uri":  p.redirect,
	}
	payload, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://oauth-login.cloud.huawei.com/oauth2/v3/token",
		bytes.NewReader(payload),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("huawei: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("huawei: 解析响应失败: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("huawei: %s", result.ErrorDesc)
	}
	return result.AccessToken, nil
}

func (p *HuaweiProvider) GetUserInfo(ctx context.Context, accessToken string) (*SocialUser, error) {
	u := fmt.Sprintf(
		"https://api.cloud.huawei.com/rest/user/v1/userinfo?access_token=%s",
		accessToken,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("huawei: 获取用户信息失败: %w", err)
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
		return nil, fmt.Errorf("huawei: 解析用户信息失败: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("huawei: %s", result.Error)
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
