package browser

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
)

const localDebugPort = "9222"

// RemoteSession 远程浏览器会话
type RemoteSession struct {
	Token      string             // 唯一标识
	AccountID  int64              // 关联账号
	PlatformID string             // 平台标识
	CreatedAt  time.Time          // 创建时间
	ExpiresAt  time.Time          // 过期时间
	BrowserCtx context.Context    // chromedp 浏览器上下文
	Cancel     context.CancelFunc // 取消函数
	LastScreen []byte             // 最新截图缓存
	ScreenMu   sync.RWMutex       // 截图锁
	Done       chan struct{}      // 登录完成信号
	Result     *SessionResult     // 登录结果
	ResultMu   sync.Mutex         // 结果锁
	IsLocal    bool               // 是否使用本地 Chrome（有窗口）
}

// SessionResult 远程登录结果
type SessionResult struct {
	Success bool
	Cookies []byte // JSON 序列化的 cookie 数据
	Error   string
}

// RemoteManager 远程浏览器会话管理器
type RemoteManager struct {
	mu        sync.RWMutex
	sessions  map[string]*RemoteSession
	remoteURL string
}

func NewRemoteManager(remoteURL string) *RemoteManager {
	return &RemoteManager{
		sessions:  make(map[string]*RemoteSession),
		remoteURL: remoteURL,
	}
}

func findLocalBrowser() string {
	candidates := []string{
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		filepath.Join(os.Getenv("LOCALAPPDATA"), `Google\Chrome\Application\chrome.exe`),
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		filepath.Join(os.Getenv("LOCALAPPDATA"), `Microsoft\Edge\Application\msedge.exe`),
		"/usr/bin/chromium-browser",
		"/usr/bin/google-chrome",
		"/usr/bin/chromium",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// fixedProfileDir 固定 profile 目录，复用已有登录态
func fixedProfileDir() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "login3rd-chrome")
}

// launchLocalChrome 启动本地 Chrome（新实例，固定 profile，可见窗口，调试端口）
// 不干扰已运行的 Chrome（独立 profile 无冲突）
func launchLocalChrome(bgCtx context.Context, browserPath string) (context.Context, context.CancelFunc, error) {
	profileDir := fixedProfileDir()
	os.MkdirAll(profileDir, 0755)

	wsURL := fmt.Sprintf("ws://127.0.0.1:%s", localDebugPort)

	// 先尝试连接（可能是之前启动的 Chrome 仍在运行）
	if allocCtx, cancel, err := dialChrome(bgCtx, wsURL); err == nil {
		fmt.Fprintf(os.Stderr, "[login3rd] 已连接 Chrome (port %s, profile=%s)\n", localDebugPort, profileDir)
		return allocCtx, cancel, nil
	}

	// 启动新 Chrome
	fmt.Fprintf(os.Stderr, "[login3rd] 启动 Chrome (port %s, profile=%s)\n", localDebugPort, profileDir)
	cmd := exec.Command(browserPath,
		"--remote-debugging-port="+localDebugPort,
		"--user-data-dir="+profileDir,
		"--no-first-run",
		"--no-default-browser-check",
	)
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("启动 Chrome 失败: %w", err)
	}

	// 等待就绪
	for i := 0; i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		if allocCtx, cancel, err := dialChrome(bgCtx, wsURL); err == nil {
			fmt.Fprintf(os.Stderr, "[login3rd] Chrome 已就绪 (port %s)\n", localDebugPort)
			return allocCtx, cancel, nil
		}
	}
	return nil, nil, fmt.Errorf("Chrome 启动超时（15s）")
}

// dialChrome 连接 Chrome 调试端口：先用 TCP 探测端口，确认 Chrome 确实在监听
func dialChrome(bgCtx context.Context, wsURL string) (context.Context, context.CancelFunc, error) {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+localDebugPort, 2*time.Second)
	if err != nil {
		return nil, nil, err
	}
	conn.Close()

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(bgCtx, wsURL)
	return allocCtx, allocCancel, nil
}

// CreateSession 创建远程浏览器会话
func (m *RemoteManager) CreateSession(ctx context.Context, accountID int64, platformID string) (*RemoteSession, error) {
	token := uuid.New().String()[:8]
	bgCtx := context.Background()

	var allocCtx context.Context
	var allocCancel context.CancelFunc
	isLocal := false

	if m.remoteURL != "" {
		allocCtx, allocCancel = chromedp.NewRemoteAllocator(bgCtx, m.remoteURL)
	} else {
		browserPath := findLocalBrowser()
		if browserPath == "" {
			return nil, fmt.Errorf("未配置 CHROME_REMOTE_URL，且本地未找到 Chrome/Edge")
		}
		var err error
		allocCtx, allocCancel, err = launchLocalChrome(bgCtx, browserPath)
		if err != nil {
			return nil, err
		}
		isLocal = true
	}

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
		Done:    make(chan struct{}),
		Result:  &SessionResult{},
		IsLocal: isLocal,
	}

	m.mu.Lock()
	m.sessions[token] = session
	m.mu.Unlock()

	if !isLocal {
		go m.screenshotLoop(session)
	}

	return session, nil
}

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

func (m *RemoteManager) DestroySession(token string) {
	m.mu.Lock()
	session, ok := m.sessions[token]
	delete(m.sessions, token)
	m.mu.Unlock()
	if ok && session.Cancel != nil {
		session.Cancel()
	}
}

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

func (m *RemoteManager) screenshotLoop(session *RemoteSession) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-session.Done:
			return
		case <-session.BrowserCtx.Done():
			return
		case <-tick.C:
			var buf []byte
			shotCtx, shotCancel := context.WithTimeout(session.BrowserCtx, 20*time.Second)
			err := chromedp.Run(shotCtx,
				chromedp.FullScreenshot(&buf, 80),
			)
			shotCancel()
			if err == nil && len(buf) > 0 {
				session.ScreenMu.Lock()
				session.LastScreen = buf
				session.ScreenMu.Unlock()
			}
		}
	}
}

func (s *RemoteSession) GetScreenshotBase64() string {
	s.ScreenMu.RLock()
	defer s.ScreenMu.RUnlock()
	if len(s.LastScreen) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(s.LastScreen)
}
