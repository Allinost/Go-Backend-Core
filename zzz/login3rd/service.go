package login3rd

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/modules/webhook"
	"github.com/Allinost/go-backend-core/zzz/login3rd/browser"
	"github.com/Allinost/go-backend-core/zzz/login3rd/provider"
)

// Service 业务逻辑层
type Service struct {
	repo          *Repository
	whSvc         *webhook.Service
	providers     map[string]provider.Provider  // platform_id -> Provider
	remoteMgr     *browser.RemoteManager
	masterKey     []byte
}

// NewService 创建 Service
func NewService(repo *Repository, whSvc *webhook.Service) *Service {
	masterKeyHex := config.Get().Crypto.MasterKeyHex
	var masterKey []byte
	if masterKeyHex != "" {
		masterKey, _ = hex.DecodeString(masterKeyHex)
	}
	if len(masterKey) == 0 {
		masterKey = make([]byte, 32) // 默认零密钥（非生产环境）
	}

	remoteURL := config.Get().Net.ChromeRemoteURL

	return &Service{
		repo:      repo,
		whSvc:     whSvc,
		providers: make(map[string]provider.Provider),
		remoteMgr: browser.NewRemoteManager(remoteURL),
		masterKey: masterKey,
	}
}

// RegisterProvider 注册 Provider
func (s *Service) RegisterProvider(p provider.Provider) {
	s.providers[p.ID()] = p
}

// ---------- Platform ----------

// ListPlatforms 列出所有平台
func (s *Service) ListPlatforms(ctx context.Context) ([]Platform, error) {
	return s.repo.ListPlatforms(ctx)
}

// ---------- Account ----------

// CreateAccount 创建账号
func (s *Service) CreateAccount(ctx context.Context, req *CreateAccountReq) (*Account, error) {
	a := &Account{
		PlatformID:     req.PlatformID,
		Label:          req.Label,
		Username:       req.Username,
		WebhookIDs:     req.WebhookIDs,
		VerifiedStatus: "unknown",
		Status:         "active",
	}

	if req.Password != "" {
		enc, err := encrypt([]byte(req.Password), s.masterKey)
		if err != nil {
			return nil, errEncryptFailed(err)
		}
		a.EncryptedPwd = enc
	}
	if req.Token != "" {
		enc, err := encrypt([]byte(req.Token), s.masterKey)
		if err != nil {
			return nil, errEncryptFailed(err)
		}
		a.EncryptedToken = enc
	}

	return s.repo.CreateAccount(ctx, a)
}

// GetAccount 获取账号
func (s *Service) GetAccount(ctx context.Context, id int64) (*Account, error) {
	a, err := s.repo.GetAccount(ctx, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, errAccountNotFound()
	}
	return a, nil
}

// ListAccounts 分页查询
func (s *Service) ListAccounts(ctx context.Context, req *ListAccountsReq) ([]Account, bool, int, error) {
	return s.repo.ListAccounts(ctx, req)
}

// UpdateAccount 更新账号
func (s *Service) UpdateAccount(ctx context.Context, id int64, req *UpdateAccountReq) (*Account, error) {
	a, err := s.repo.GetAccount(ctx, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, errAccountNotFound()
	}

	updates := map[string]any{}
	if req.Label != nil {
		updates["label"] = *req.Label
	}
	if req.Username != nil {
		updates["username"] = *req.Username
	}
	if req.Password != nil {
		enc, err := encrypt([]byte(*req.Password), s.masterKey)
		if err != nil {
			return nil, errEncryptFailed(err)
		}
		updates["encrypted_password"] = enc
	}
	if req.Token != nil {
		enc, err := encrypt([]byte(*req.Token), s.masterKey)
		if err != nil {
			return nil, errEncryptFailed(err)
		}
		updates["encrypted_token"] = enc
	}
	if req.WebhookIDs != nil {
		updates["webhook_ids"] = req.WebhookIDs
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if err := s.repo.UpdateAccount(ctx, id, updates); err != nil {
		return nil, err
	}
	return s.repo.GetAccount(ctx, id)
}

// DeleteAccount 删除账号
func (s *Service) DeleteAccount(ctx context.Context, id int64) error {
	return s.repo.DeleteAccount(ctx, id)
}

// GetAccountStatus 获取账号状态详情
func (s *Service) GetAccountStatus(ctx context.Context, id int64) (*AccountStatusResp, error) {
	a, err := s.repo.GetAccount(ctx, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, errAccountNotFound()
	}

	p, err := s.repo.GetPlatform(ctx, a.PlatformID)
	if err != nil {
		return nil, err
	}

	resp := &AccountStatusResp{
		Account:  *a,
		Platform: *p,
		SessionValid: a.VerifiedStatus == "valid" && (a.SessionExpiresAt == nil || a.SessionExpiresAt.After(time.Now())),
		CanAutoLogin: p.HasAuto && a.EncryptedPwd != "" && s.providers[a.PlatformID] != nil,
	}

	if a.SessionExpiresAt != nil {
		resp.SessionExpiresIn = time.Until(*a.SessionExpiresAt).Round(time.Second).String()
	}

	return resp, nil
}

// ---------- Login ----------

// AutoLogin 自动登录（使用账号密码）
func (s *Service) AutoLogin(ctx context.Context, accountID int64) (*Account, error) {
	a, err := s.repo.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, errAccountNotFound()
	}
	if a.Status == "disabled" {
		return nil, errAccountDisabled()
	}

	p, err := s.repo.GetPlatform(ctx, a.PlatformID)
	if err != nil || p == nil {
		return nil, errPlatformNotFound()
	}
	if !p.HasAuto {
		return nil, errAutoLoginNotSupported()
	}

	prov, ok := s.providers[a.PlatformID]
	if !ok {
		return nil, errAutoLoginNotSupported()
	}

	// 解密密码
	pwdBytes, err := decrypt(a.EncryptedPwd, s.masterKey)
	if err != nil {
		return nil, errDecryptFailed(err)
	}

	sessionData, err := prov.AutoLogin(ctx, a.Username, string(pwdBytes))
	if err != nil {
		s.repo.UpdateAccount(ctx, accountID, map[string]any{"last_error": err.Error()})
		return nil, errAutoLoginFailed(err)
	}

	// 更新 session
	var expiresAt *time.Time
	if sessionData.ExpiresAt > 0 {
		t := time.Unix(sessionData.ExpiresAt, 0)
		expiresAt = &t
	}
	if err := s.repo.UpdateSession(ctx, accountID, toModelSession(sessionData), expiresAt, "auto"); err != nil {
		return nil, err
	}

	return s.repo.GetAccount(ctx, accountID)
}

// CreateRemoteSession 创建远程浏览器会话（用于 webhook 链接）
func (s *Service) CreateRemoteSession(ctx context.Context, accountID int64) (string, error) {
	a, err := s.repo.GetAccount(ctx, accountID)
	if err != nil {
		return "", err
	}
	if a == nil {
		return "", errAccountNotFound()
	}

	session, err := s.remoteMgr.CreateSession(ctx, accountID, a.PlatformID)
	if err != nil {
		return "", errRemoteSessionCreateFailed(err)
	}

	// 导航到登录页
	p, _ := s.repo.GetPlatform(ctx, a.PlatformID)
	if p != nil && p.LoginURL != "" {
		formCfg := (*browser.FormConfig)(nil)
		if p.FormCfg != nil {
			formCfg = &browser.FormConfig{
				TabSelector:       p.FormCfg.TabSelector,
				UsernameSelector:  p.FormCfg.UsernameSelector,
				PasswordSelector:  p.FormCfg.PasswordSelector,
				SubmitSelector:    p.FormCfg.SubmitSelector,
				CaptchaSelector:   p.FormCfg.CaptchaSelector,
				SuccessCookies:    p.FormCfg.SuccessCookies,
				SuccessURLPrefix:  p.FormCfg.SuccessURLPrefix,
				LoginWaitSelector: p.FormCfg.LoginWaitSelector,
				MaxAutoRetry:      p.FormCfg.MaxAutoRetry,
			}
		}
		flow := &browser.LoginFlow{
			LoginURL: p.LoginURL,
			HomeURL:  p.HomeURL,
			FormCfg:  formCfg,
		}
		go func() {
			pwd := ""
			if a.EncryptedPwd != "" {
				if pwdBytes, err := decrypt(a.EncryptedPwd, s.masterKey); err == nil {
					pwd = string(pwdBytes)
				}
			}
			sessionData, err := flow.Execute(context.Background(), session.BrowserCtx, a.Username, pwd)
			if err != nil {
				s.remoteMgr.MarkDone(session.Token, &browser.SessionResult{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			// 更新 DB
			s.repo.UpdateSession(context.Background(), accountID, toModelSession(sessionData), nil, "remote")
			s.remoteMgr.MarkDone(session.Token, &browser.SessionResult{
				Success: true,
			})
		}()
	}

	return session.Token, nil
}

// VerifySession 验证账号 session 是否有效
func (s *Service) VerifySession(ctx context.Context, accountID int64) (bool, error) {
	a, err := s.repo.GetAccount(ctx, accountID)
	if err != nil {
		return false, err
	}
	if a == nil {
		return false, errAccountNotFound()
	}
	if a.SessionData == nil {
		return false, nil
	}

	prov, ok := s.providers[a.PlatformID]
	if !ok {
		return false, nil
	}

	valid, err := prov.Verify(ctx, toProviderSession(a.SessionData))
	if err != nil {
		return false, err
	}

	status := "valid"
	if !valid {
		status = "expired"
	}
	s.repo.UpdateVerifiedStatus(ctx, accountID, status)

	return valid, nil
}

// RefreshSession 尝试刷新 session
func (s *Service) RefreshSession(ctx context.Context, accountID int64) (*Account, error) {
	a, err := s.repo.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, errAccountNotFound()
	}

	prov, ok := s.providers[a.PlatformID]
	if !ok || a.SessionData == nil {
		return a, nil
	}

	newSession, err := prov.Refresh(ctx, toProviderSession(a.SessionData))
	if err != nil {
		return a, nil
	}

	var expiresAt *time.Time
	if newSession.ExpiresAt > 0 {
		t := time.Unix(newSession.ExpiresAt, 0)
		expiresAt = &t
	}
	if err := s.repo.UpdateSession(ctx, accountID, toModelSession(newSession), expiresAt, "refresh"); err != nil {
		return nil, err
	}

	return s.repo.GetAccount(ctx, accountID)
}

// Logout 登出（清除 session 数据，重置状态）
func (s *Service) Logout(ctx context.Context, accountID int64) error {
	a, err := s.repo.GetAccount(ctx, accountID)
	if err != nil {
		return err
	}
	if a == nil {
		return errAccountNotFound()
	}
	return s.repo.UpdateAccount(ctx, accountID, map[string]any{
		"session_data":     nil,
		"session_expires_at": nil,
		"verified_status":  "unknown",
		"login_method":     "",
		"last_error":       "用户手动登出",
	})
}

// SendWebhookNotification 发送 webhook 通知
func (s *Service) SendWebhookNotification(ctx context.Context, accountID int64, title, content string) error {
	a, err := s.repo.GetAccount(ctx, accountID)
	if err != nil || a == nil {
		return err
	}
	if len(a.WebhookIDs) == 0 {
		return nil
	}

	p, _ := s.repo.GetPlatform(ctx, a.PlatformID)
	loginURL := ""
	if p != nil {
		loginURL = p.LoginURL
	}

	_, err = s.whSvc.Send(ctx, &webhook.SendReq{
		WebhookIDs: a.WebhookIDs,
		Source:     "login3rd",
		Title:      title,
		Content:    content,
		MsgType:    "text",
		Data: map[string]any{
			"platform":   a.PlatformID,
			"account_id": accountID,
			"login_url":  loginURL,
		},
	})
	return err
}

// GetRemoteManager 返回远程浏览器管理器
func (s *Service) GetRemoteManager() *browser.RemoteManager {
	return s.remoteMgr
}

// GetProvider 按平台获取 Provider
func (s *Service) GetProvider(platformID string) provider.Provider {
	return s.providers[platformID]
}

// GetDecryptedPassword 解密账号密码（供 browser login_flow 使用）
func (s *Service) GetDecryptedPassword(a *Account) string {
	if a.EncryptedPwd == "" {
		return ""
	}
	pwdBytes, err := decrypt(a.EncryptedPwd, s.masterKey)
	if err != nil {
		return ""
	}
	return string(pwdBytes)
}

// ---------- 类型转换 ----------

func toModelSession(s *provider.SessionData) *SessionData {
	if s == nil {
		return nil
	}
	cookies := make([]SessionCookie, len(s.Cookies))
	for i, c := range s.Cookies {
		cookies[i] = SessionCookie{
			Name: c.Name, Value: c.Value, Domain: c.Domain,
			Path: c.Path, Secure: c.Secure, HTTPOnly: c.HTTPOnly,
		}
	}
	var expiresAt *time.Time
	if s.ExpiresAt > 0 {
		t := time.Unix(s.ExpiresAt, 0)
		expiresAt = &t
	}
	return &SessionData{
		Cookies:   cookies,
		Headers:   s.Headers,
		Extra:     s.Extra,
		ExpiresAt: expiresAt,
	}
}

func toProviderSession(s *SessionData) *provider.SessionData {
	if s == nil {
		return nil
	}
	cookies := make([]provider.Cookie, len(s.Cookies))
	for i, c := range s.Cookies {
		expires := int64(0)
		if c.Expires.IsZero() == false {
			expires = c.Expires.Unix()
		}
		cookies[i] = provider.Cookie{
			Name: c.Name, Value: c.Value, Domain: c.Domain,
			Path: c.Path, Secure: c.Secure, HTTPOnly: c.HTTPOnly,
			Expires: expires,
		}
	}
	return &provider.SessionData{
		Cookies: cookies,
		Headers: s.Headers,
		Extra:   s.Extra,
	}
}

// ---------- Chrome 远程 URL 配置获取 ----------

// GetChromeRemoteURL 获取 Chrome 远程连接地址
func (s *Service) GetChromeRemoteURL() string {
	return config.Get().Net.ChromeRemoteURL
}
