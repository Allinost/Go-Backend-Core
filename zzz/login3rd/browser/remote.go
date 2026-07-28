package browser

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
)

// RemoteSession 远程浏览器会话
type RemoteSession struct {
	Token      string                   // 唯一标识
	AccountID  int64                    // 关联账号
	PlatformID string                   // 平台标识
	CreatedAt  time.Time                // 创建时间
	ExpiresAt  time.Time                // 过期时间
	BrowserCtx context.Context          // chromedp 浏览器上下文
	Cancel     context.CancelFunc       // 取消函数
	LastScreen []byte                   // 最新截图缓存
	ScreenMu   sync.RWMutex             // 截图锁
	Done       chan struct{}            // 登录完成信号
	Result     *SessionResult           // 登录结果
	ResultMu   sync.Mutex               // 结果锁
}

// SessionResult 远程登录结果
type SessionResult struct {
	Success   bool
	Cookies   []byte // JSON 序列化的 cookie 数据
	Error     string
}

// RemoteManager 远程浏览器会话管理器
type RemoteManager struct {
	mu        sync.RWMutex
	sessions  map[string]*RemoteSession
	remoteURL string // browserless/chrome 连接地址
}

// NewRemoteManager 创建管理器
func NewRemoteManager(remoteURL string) *RemoteManager {
	return &RemoteManager{
		sessions:  make(map[string]*RemoteSession),
		remoteURL: remoteURL,
	}
}

// CreateSession 创建远程浏览器会话
func (m *RemoteManager) CreateSession(ctx context.Context, accountID int64, platformID string) (*RemoteSession, error) {
	token := uuid.New().String()[:8]

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, m.remoteURL)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)

	session := &RemoteSession{
		Token:      token,
		AccountID:  accountID,
		PlatformID: platformID,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(5 * time.Minute),
		BrowserCtx: browserCtx,
		Cancel: func() {
			allocCancel()
			browserCancel()
		},
		Done:   make(chan struct{}),
		Result: &SessionResult{},
	}

	m.mu.Lock()
	m.sessions[token] = session
	m.mu.Unlock()

	// 启动截图 goroutine
	go m.screenshotLoop(session)

	return session, nil
}

// GetSession 获取远程会话
func (m *RemoteManager) GetSession(token string) (*RemoteSession, error) {
	m.mu.RLock()
	session, ok := m.sessions[token]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("会话不存在")
	}
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("会话已过期")
	}
	return session, nil
}

// DestroySession 销毁远程会话
func (m *RemoteManager) DestroySession(token string) {
	m.mu.Lock()
	session, ok := m.sessions[token]
	delete(m.sessions, token)
	m.mu.Unlock()
	if ok && session.Cancel != nil {
		session.Cancel()
	}
}

// MarkDone 标记登录完成
func (m *RemoteManager) MarkDone(token string, result *SessionResult) {
	m.mu.RLock()
	session, ok := m.sessions[token]
	m.mu.RUnlock()
	if !ok {
		return
	}
	session.ResultMu.Lock()
	session.Result = result
	session.ResultMu.Unlock()
	close(session.Done)
}

// CleanupExpired 清理过期会话
func (m *RemoteManager) CleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for token, session := range m.sessions {
		if time.Now().After(session.ExpiresAt) {
			if session.Cancel != nil {
				session.Cancel()
			}
			delete(m.sessions, token)
		}
	}
}

// screenshotLoop 定期截取浏览器截图
func (m *RemoteManager) screenshotLoop(session *RemoteSession) {
	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-session.Done:
			return
		case <-session.BrowserCtx.Done():
			return
		case <-tick.C:
			var buf []byte
			err := chromedp.Run(session.BrowserCtx,
				chromedp.FullScreenshot(&buf, 80),
			)
			if err == nil && len(buf) > 0 {
				session.ScreenMu.Lock()
				session.LastScreen = buf
				session.ScreenMu.Unlock()
			}
		}
	}
}

// GetScreenshotBase64 获取最新截图（base64 编码）
func (s *RemoteSession) GetScreenshotBase64() string {
	s.ScreenMu.RLock()
	defer s.ScreenMu.RUnlock()
	if len(s.LastScreen) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(s.LastScreen)
}
