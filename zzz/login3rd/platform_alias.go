package login3rd

import (
	"context"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/zzz/login3rd/provider"
)

// SessionPlatformCandidates 返回可共享同一登录态的平台顺序。
// 京东/七鲜/京东移动版共用 pt_key/pt_pin；淘宝/天猫共用 unb/_tb_token_ 等 Cookie。
func SessionPlatformCandidates(platformID string) []string {
	id := strings.ToLower(strings.TrimSpace(platformID))
	if id == "" {
		return nil
	}

	var candidates []string
	switch id {
	case "jd", "qixian", "jdm":
		candidates = []string{id, "jd", "qixian", "jdm"}
	case "taobao", "tmall":
		candidates = []string{id, "taobao", "tmall"}
	default:
		candidates = []string{id}
	}

	seen := make(map[string]bool, len(candidates))
	result := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		result = append(result, c)
	}
	return result
}

// GetValidSessionAccount 按平台别名为采集器解析可用登录账号。
// 先找平台自身账号，再回退到共享登录的兄弟平台（如七鲜回退京东、天猫回退淘宝）。
func (s *Service) GetValidSessionAccount(ctx context.Context, platformID string) (*Account, error) {
	for _, candidate := range SessionPlatformCandidates(platformID) {
		accounts, err := s.repo.GetAccountByPlatform(ctx, candidate)
		if err != nil {
			return nil, err
		}
		for i := range accounts {
			if accountSessionUsable(&accounts[i]) {
				return &accounts[i], nil
			}
		}
	}
	return nil, nil
}

// HasValidSession 判断平台或其共享登录平台是否存在可用会话。
func (s *Service) HasValidSession(ctx context.Context, platformID string) bool {
	acct, err := s.GetValidSessionAccount(ctx, platformID)
	return err == nil && acct != nil
}

// GetSessionCookies 返回平台（含共享登录平台）当前可用会话 Cookie。
func (s *Service) GetSessionCookies(ctx context.Context, platformID string) []*provider.Cookie {
	acct, err := s.GetValidSessionAccount(ctx, platformID)
	if err != nil || acct == nil || acct.SessionData == nil {
		return nil
	}
	cookies := make([]*provider.Cookie, 0, len(acct.SessionData.Cookies))
	for _, c := range acct.SessionData.Cookies {
		cookies = append(cookies, &provider.Cookie{
			Name: c.Name, Value: c.Value, Domain: c.Domain,
			Path: c.Path, Secure: c.Secure, HTTPOnly: c.HTTPOnly,
		})
	}
	return cookies
}

// accountSessionUsable 判断账号是否拥有可直接用于采集的登录态。
func accountSessionUsable(a *Account) bool {
	if a == nil || a.Status == "disabled" {
		return false
	}
	if a.SessionData == nil || len(a.SessionData.Cookies) == 0 {
		return false
	}
	if a.VerifiedStatus != "valid" {
		return false
	}
	if a.SessionExpiresAt != nil && a.SessionExpiresAt.Before(time.Now()) {
		return false
	}
	return true
}
