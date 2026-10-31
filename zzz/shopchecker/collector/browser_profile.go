package collector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/chromedp/chromedp"
)

var profileUnsafeRe = regexp.MustCompile(`[^a-z0-9._-]+`)

var browserProfileLocks sync.Map // profile name -> chan struct{}（容量 1 的互斥锁）

// browserProfileName 归一化平台名。京东系/淘系共用账号，必须共用 Profile。
func browserProfileName(platform string) string {
	name := profileUnsafeRe.ReplaceAllString(strings.ToLower(strings.TrimSpace(platform)), "_")
	name = strings.Trim(name, "._-")
	switch name {
	case "qixian", "jdm", "jd":
		name = "jd"
	case "tmall", "taobao":
		name = "taobao"
	}
	if name == "" {
		name = "default"
	}
	return name
}

// acquireBrowserProfile 串行化同一 Profile 的浏览器启动，避免 Chrome 争抢 UserDataDir。
func acquireBrowserProfile(ctx context.Context, platform string) (func(), error) {
	v, _ := browserProfileLocks.LoadOrStore(browserProfileName(platform), make(chan struct{}, 1))
	ch := v.(chan struct{})
	select {
	case ch <- struct{}{}:
		return func() { <-ch }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// browserProfileDir 返回平台专属的持久化 Chrome Profile 目录。
//
// 每次采集都新建临时 Profile 会被平台风控识别为“新设备”，京东尤其敏感：
// 同一账号短时间内在多台“新设备”上访问，很容易触发频控（商品页 403、购物车/搜索被限制）。
// 复用固定 Profile 后，Cookie、LocalStorage、设备指纹、浏览历史都能延续。
//
// 可通过环境变量调整：
//   - SHOPCHECKER_BROWSER_PROFILE_DIR：Profile 根目录，默认 <用户缓存目录>/go-backend-core/browser-profiles
//   - SHOPCHECKER_BROWSER_PROFILE=temp：退回旧的临时 Profile 行为（排查 Profile 锁问题时使用）
func browserProfileDir(platform string) (string, error) {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("SHOPCHECKER_BROWSER_PROFILE")), "temp") {
		dir, err := os.MkdirTemp("", platform+"-scrape-*")
		if err != nil {
			return "", fmt.Errorf("创建临时浏览器目录失败: %w", err)
		}
		return dir, nil
	}

	base := strings.TrimSpace(os.Getenv("SHOPCHECKER_BROWSER_PROFILE_DIR"))
	if base == "" {
		cacheDir, err := os.UserCacheDir()
		if err != nil || cacheDir == "" {
			cacheDir = os.TempDir()
		}
		base = filepath.Join(cacheDir, "go-backend-core", "browser-profiles")
	}

	name := browserProfileName(platform)
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("创建浏览器 Profile 目录失败: %w", err)
	}
	return dir, nil
}

// newBrowserWithProfile 使用平台固定 Profile 启动 Chrome，并串行化同一 Profile 的使用。
func newBrowserWithProfile(ctx context.Context, browserPath, platform string, extraOpts ...chromedp.ExecAllocatorOption) (context.Context, context.CancelFunc, error) {
	profileDir, err := browserProfileDir(platform)
	if err != nil {
		return nil, nil, err
	}
	unlock, err := acquireBrowserProfile(ctx, platform)
	if err != nil {
		return nil, nil, err
	}
	fmt.Fprintf(os.Stderr, "[%s] 使用浏览器 Profile: %s\n", platform, profileDir)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(browserPath),
		chromedp.NoSandbox,
		chromedp.UserDataDir(profileDir),
	)
	opts = append(opts, extraOpts...)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	tabCtx, tabCancel := chromedp.NewContext(allocCtx)
	select {
	case <-tabCtx.Done():
		tabCancel()
		allocCancel()
		unlock()
		return nil, nil, fmt.Errorf("本地浏览器启动失败（%s）: %w", browserPath, tabCtx.Err())
	default:
	}
	return tabCtx, func() {
		tabCancel()
		allocCancel()
		unlock()
	}, nil
}
