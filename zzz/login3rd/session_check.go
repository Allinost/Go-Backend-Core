package login3rd

import (
	"context"
	"fmt"

	"github.com/Allinost/go-backend-core/internal/pkg/logger"
)

// RunSessionCheck 执行一次完整的 session 健康检查
func RunSessionCheck(ctx context.Context, svc *Service) {
	accounts, _, _, err := svc.ListAccounts(ctx, &ListAccountsReq{
		Page:     1,
		PageSize: 100,
	})
	if err != nil {
		logger.Error().Err(err).Msg("login3rd: session 检查获取账号列表失败")
		return
	}

	for _, a := range accounts {
		if a.Status == "disabled" || a.Status == "expired" {
			continue
		}

		func(acct Account) {
			valid, err := svc.VerifySession(ctx, acct.ID)
			if err != nil {
				logger.Error().Err(err).Int64("account_id", acct.ID).Msg("login3rd: session 验证失败")
				return
			}

			if valid {
				return
			}

			logger.Warn().Int64("account_id", acct.ID).Str("platform", acct.PlatformID).Msg("login3rd: session 已过期，尝试刷新")

			refreshed, err := svc.RefreshSession(ctx, acct.ID)
			if err != nil {
				logger.Error().Err(err).Int64("account_id", acct.ID).Msg("login3rd: session 刷新失败")
			}
			if refreshed != nil && refreshed.VerifiedStatus == "valid" {
				logger.Info().Int64("account_id", acct.ID).Msg("login3rd: session 刷新成功")
				return
			}

			auto, err := svc.AutoLogin(ctx, acct.ID)
			if err == nil && auto != nil && auto.VerifiedStatus == "valid" {
				logger.Info().Int64("account_id", acct.ID).Msg("login3rd: 自动登录成功")
				return
			}

			remoteToken := ""
			if token, err := svc.CreateRemoteSession(ctx, acct.ID); err == nil {
				remoteToken = token
			} else {
				logger.Error().Err(err).Int64("account_id", acct.ID).Msg("login3rd: 创建远程浏览器会话失败")
			}

			svc.UpdateAccount(ctx, acct.ID, &UpdateAccountReq{
				Status: strPtr("expired"),
			})
			logger.Warn().Int64("account_id", acct.ID).Str("platform", acct.PlatformID).Msg("login3rd: 无法续期，已标记过期")

			title := fmt.Sprintf("[Login3rd] %s 登录已过期", acct.Label)
			content := ""
			if remoteToken != "" {
				content = fmt.Sprintf(
					"账号「%s」（%s）的登录会话已过期。\n点击下方链接在浏览器中完成登录（5分钟内有效）：\n%s/api/v1/login3rd/remote/session/%s",
					acct.Label, acct.PlatformID, getHost(), remoteToken,
				)
			} else {
				content = fmt.Sprintf(
					"账号「%s」（%s）的登录会话已过期，自动续期失败。", acct.Label, acct.PlatformID,
				)
			}
			if err := svc.SendWebhookNotification(ctx, acct.ID, title, content); err != nil {
				logger.Error().Err(err).Int64("account_id", acct.ID).Msg("login3rd: webhook 通知发送失败")
			}
		}(a)
	}
}

// SessionChecker 会话健康检查调度器（通过 centralized scheduler 驱动）
type SessionChecker struct {
	svc    *Service
	stopCh chan struct{}
}

// NewSessionChecker 创建 SessionChecker
func NewSessionChecker(svc *Service) *SessionChecker {
	return &SessionChecker{
		svc:    svc,
		stopCh: make(chan struct{}),
	}
}

// Start 启动定时检查
func (s *SessionChecker) Start() {
	logger.Warn().Msg("login3rd: SessionChecker.Start() 已弃用，改为 centralized scheduler 驱动")
}

// Stop 停止
func (s *SessionChecker) Stop() {
	close(s.stopCh)
}

func getHost() string {
	return "https://et.hailong.site:29090"
}

func strPtr(s string) *string { return &s }
