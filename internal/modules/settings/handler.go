package settings

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	appErrors "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/Allinost/go-backend-core/internal/services/audit"
	"github.com/Allinost/go-backend-core/zzz/login3rd"
	"github.com/gin-gonic/gin"
)

const (
	defaultN8nURL      = "http://n8n:5678"
	defaultOpenClawURL = "http://openclaw:18789"
	defaultLangBotURL  = "http://langbot:5300"
)

// StatusItem 依赖健康状态
type StatusItem struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// Meta 只读元信息
type Meta struct {
	Version         string   `json:"version"`
	Mode            string   `json:"mode"`
	ServerName      string   `json:"server_name"`
	SettingsFile    string   `json:"settings_file"`
	Persisted       bool     `json:"persisted"`
	ChromeRemoteURL string   `json:"chrome_remote_url"`
	ReadOnlyKeys    []string `json:"read_only_keys"`
}

// View 设置读取响应
type View struct {
	Settings Data         `json:"settings"`
	Status   []StatusItem `json:"status"`
	Meta     Meta         `json:"meta"`
}

// Handler 设置 HTTP 处理器
type Handler struct {
	store *Store
	cfg   *config.Config
}

// NewHandler 创建设置处理器
func NewHandler(store *Store, cfg *config.Config) *Handler {
	return &Handler{store: store, cfg: cfg}
}

// defaults 文件不存在时的默认设置
func defaults(cfg *config.Config) Data {
	return Data{
		PublicBaseURL:   cfg.Server.PublicBaseURL,
		FrontendTitle:   cfg.Server.Name,
		FrontendAPIBase: "/api/v1",
		BrowserURL:      cfg.Net.ChromeRemoteURL,
		N8nURL:          defaultN8nURL,
		OpenClawURL:     defaultOpenClawURL,
		LangBotURL:      defaultLangBotURL,
	}
}

// Get 获取当前设置与依赖状态
// @Summary      获取应用设置与依赖状态
// @Tags         settings
// @Produce      json
// @Success      200  {object}  response.Response{data=View}
// @Router       /settings [get]
// @Security     BearerAuth
func (h *Handler) Get(c *gin.Context) {
	view, err := h.buildView()
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}
	response.Success(c, view)
}

// Update 更新应用设置（管理员）
// @Summary      更新应用设置
// @Tags         settings
// @Accept       json
// @Produce      json
// @Param        body  body  Data  true  "应用设置"
// @Success      200  {object}  response.Response{data=View}
// @Router       /settings [put]
// @Security     BearerAuth
func (h *Handler) Update(c *gin.Context) {
	var req Data
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	normalized, err := normalizeData(req)
	if err != nil {
		response.ParamErr(c, err.Error())
		return
	}
	if err := h.store.Save(normalized); err != nil {
		logger.Error().Err(err).Msg("settings: 保存设置失败")
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}

	// 运行时应用公开地址与 Browser 地址
	applyRuntime(h.cfg, normalized)

	uid := ""
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(uint); ok {
			uid = strconv.FormatUint(uint64(id), 10)
		}
	}
	username, _ := c.Get("username")
	if err := audit.RecordUpdate(c.Request.Context(), uid, fmt.Sprint(username), "settings", "应用设置更新"); err != nil {
		logger.Warn().Err(err).Msg("settings: 审计记录失败")
	}

	view, err := h.buildView()
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}
	response.Success(c, view)
}

// buildView 组装设置、依赖状态与元信息
func (h *Handler) buildView() (*View, error) {
	data, err := h.store.Load(defaults(h.cfg))
	if err != nil {
		return nil, err
	}
	health := database.Health()
	items := make([]StatusItem, 0, len(health))
	for name, st := range health {
		items = append(items, StatusItem{Name: name, Status: st.Status})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })

	meta := Meta{
		Version:         h.cfg.Server.Version,
		Mode:            h.cfg.Server.Mode,
		ServerName:      h.cfg.Server.Name,
		SettingsFile:    h.store.Path(),
		Persisted:       h.store.Exists(),
		ChromeRemoteURL: h.cfg.Net.ChromeRemoteURL,
		ReadOnlyKeys:    []string{"mysql", "redis", "jwt", "postgres", "rustfs"},
	}
	return &View{Settings: data, Status: items, Meta: meta}, nil
}

// normalizeData 校验并归一化设置
func normalizeData(in Data) (Data, error) {
	out := Data{
		FrontendTitle: strings.TrimSpace(in.FrontendTitle),
	}
	var err error
	if out.PublicBaseURL, err = normalizeURL(in.PublicBaseURL, true, "http", "https"); err != nil {
		return out, fmt.Errorf("公开地址不合法: %w", err)
	}

	apiBase := strings.TrimSpace(in.FrontendAPIBase)
	if apiBase == "" {
		apiBase = "/api/v1"
	}
	if strings.HasPrefix(apiBase, "/") {
		out.FrontendAPIBase = strings.TrimSuffix(apiBase, "/")
	} else {
		if out.FrontendAPIBase, err = normalizeURL(apiBase, false, "http", "https"); err != nil {
			return out, fmt.Errorf("前端 API 地址不合法: %w", err)
		}
	}

	if out.BrowserURL, err = normalizeURL(in.BrowserURL, true, "ws", "wss", "http", "https"); err != nil {
		return out, fmt.Errorf("Browser 地址不合法: %w", err)
	}
	if out.N8nURL, err = normalizeURL(in.N8nURL, true, "http", "https", "ws", "wss"); err != nil {
		return out, fmt.Errorf("n8n 地址不合法: %w", err)
	}
	if out.OpenClawURL, err = normalizeURL(in.OpenClawURL, true, "http", "https", "ws", "wss"); err != nil {
		return out, fmt.Errorf("OpenClaw 地址不合法: %w", err)
	}
	if out.LangBotURL, err = normalizeURL(in.LangBotURL, true, "http", "https", "ws", "wss"); err != nil {
		return out, fmt.Errorf("LangBot 地址不合法: %w", err)
	}
	return out, nil
}

// applyRuntime 应用可在线生效的设置
func applyRuntime(cfg *config.Config, data Data) {
	cfg.Server.PublicBaseURL = data.PublicBaseURL
	login3rd.SetPublicBaseURL(data.PublicBaseURL)

	cfg.Net.ChromeRemoteURL = data.BrowserURL
	if data.BrowserURL != "" {
		_ = os.Setenv("CHROME_REMOTE_URL", data.BrowserURL)
	} else {
		_ = os.Unsetenv("CHROME_REMOTE_URL")
	}
}

// ApplyEarly 在模块初始化前应用已持久化设置（公开地址、Browser 地址），
// 保证 login3rd/shopchecker 启动时读到的是设置页保存的值。
func ApplyEarly(cfg *config.Config) {
	store := NewStore(settingsFilePath())
	data, err := store.Load(defaults(cfg))
	if err != nil {
		return
	}
	if data.PublicBaseURL != "" {
		cfg.Server.PublicBaseURL = data.PublicBaseURL
	}
	if data.BrowserURL != "" {
		cfg.Net.ChromeRemoteURL = data.BrowserURL
		_ = os.Setenv("CHROME_REMOTE_URL", data.BrowserURL)
	}
}

// normalizeURL 校验 URL；空字符串允许时返回空
func normalizeURL(raw string, allowEmpty bool, schemes ...string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if allowEmpty {
			return "", nil
		}
		return "", fmt.Errorf("不能为空")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("格式错误")
	}
	allowed := false
	for _, s := range schemes {
		if strings.EqualFold(u.Scheme, s) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("协议必须是 %s", strings.Join(schemes, "/"))
	}
	return strings.TrimSuffix(raw, "/"), nil
}
