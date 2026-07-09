package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/middleware"
	appErrors "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/Allinost/go-backend-core/internal/services/auth"
	"github.com/Allinost/go-backend-core/internal/services/eventbus"
	"github.com/gin-gonic/gin"
)

type Module struct {
	svc       *auth.Service
	user      auth.UserStore
	blacklist auth.TokenBlacklist
	bus       eventbus.EventBus
	rbac      *auth.RBACService
	providers map[string]auth.SocialProvider
	social    auth.SocialStore
}

func (m *Module) Name() string {
	return "auth"
}

func (m *Module) Init(cfg *config.Config) error {
	accessExpire, _ := time.ParseDuration(cfg.Auth.JWTExpire)
	if accessExpire <= 0 {
		accessExpire = 30 * time.Minute
	}
	refreshExpire := 7 * 24 * time.Hour

	m.svc = auth.NewService(auth.Config{
		Secret:        cfg.Auth.JWTSecret,
		AccessExpire:  accessExpire,
		RefreshExpire: refreshExpire,
	})

	if database.DB != nil {
		if pool, ok := database.DB.MySQL["main"]; ok && pool.DB != nil {
			store, err := auth.NewMySQLUserStore(pool.DB)
			if err == nil {
				_ = store.AutoMigrate()
				m.user = store
			}
		}
	}
	if m.user == nil {
		m.user = auth.NewInMemoryUserStore()
	}

	if database.DB != nil {
		if rdb := database.GetRedis("main"); rdb != nil {
			m.blacklist = auth.NewRedisBlacklist(rdb)
		}
	}
	if m.blacklist == nil {
		m.blacklist = auth.NewInMemoryBlacklist()
	}

	m.social = auth.NewInMemorySocialStore()

	var rbacStore auth.RBACStore
	if database.DB != nil {
		if pool, ok := database.DB.MySQL["main"]; ok && pool.DB != nil {
			mysqlStore, err := auth.NewMySQLRBACStore(pool.DB)
			if err == nil {
				_ = mysqlStore.AutoMigrate()
				_ = mysqlStore.EnsureDefaultPermissions()
				rbacStore = mysqlStore
			}
		}
	}
	if rbacStore == nil {
		rbacStore = auth.NewInMemoryRBACStore()
		_ = rbacStore.EnsureDefaultPermissions()
	}
	m.rbac = auth.NewRBACService(rbacStore)

	m.providers = make(map[string]auth.SocialProvider)
	m.registerProviders(cfg)

	return nil
}

func (m *Module) registerProviders(cfg *config.Config) {
	o := cfg.Auth.OAuth2
	if o.Wechat != nil {
		m.RegisterProvider(auth.NewWechatProvider(o.Wechat.ClientID, o.Wechat.ClientSecret, o.Wechat.RedirectURL))
	}
	if o.Feishu != nil {
		m.RegisterProvider(auth.NewFeishuProvider(o.Feishu.ClientID, o.Feishu.ClientSecret, o.Feishu.RedirectURL))
	}
	if o.QQ != nil {
		m.RegisterProvider(auth.NewQQProvider(o.QQ.ClientID, o.QQ.ClientSecret, o.QQ.RedirectURL))
	}
	if o.Apple != nil {
		m.RegisterProvider(auth.NewAppleProvider(o.Apple.ClientID, o.Apple.TeamID, o.Apple.KeyID, o.Apple.PrivateKey, o.Apple.RedirectURL))
	}
	if o.Huawei != nil {
		m.RegisterProvider(auth.NewHuaweiProvider(o.Huawei.ClientID, o.Huawei.ClientSecret, o.Huawei.RedirectURL))
	}
	if o.Honor != nil {
		m.RegisterProvider(auth.NewHonorProvider(o.Honor.ClientID, o.Honor.ClientSecret, o.Honor.RedirectURL))
	}
}

func (m *Module) RegisterProvider(p auth.SocialProvider) {
	m.providers[p.Name()] = p
}

func (m *Module) Close() error {
	return nil
}

func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/register", m.register)
	r.POST("/login", m.login)
	r.POST("/refresh", m.refresh)

	for name, provider := range m.providers {
		p := provider
		r.GET("/"+name+"/url", m.authURL(p))
		r.POST("/"+name+"/callback", m.callback(p))
	}

	protected := r.Group("")
	protected.Use(middleware.AuthRequiredWithBlacklist(m.svc, m.blacklist))
	protected.GET("/me", m.me)
	protected.POST("/logout", m.logout)
	protected.GET("/accounts", m.listAccounts)

	for name := range m.providers {
		n := name
		protected.POST("/"+n+"/bind", m.bind(n))
		protected.POST("/"+n+"/unbind", m.unbind(n))
	}

	protected.POST("/roles/assign", middleware.RequirePermission(m.rbac, "user", "admin"), m.assignRole)
}

func (m *Module) register(c *gin.Context) {
	var req auth.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "请求格式错误: "+err.Error())
		return
	}

	user, err := m.user.CreateUser(req)
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeConflict, err.Error()))
		return
	}

	m.rbac.AssignRole(user.ID, string(auth.RoleUser))

	pair, err := m.svc.GenerateTokenPair(user.ID, user.Username)
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, "token 签发失败"))
		return
	}

	m.publishEvent("user.registered", user.ID, c.ClientIP())
	response.Success(c, pair)
}

func (m *Module) login(c *gin.Context) {
	var req auth.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "请求格式错误: "+err.Error())
		return
	}

	user, err := m.user.VerifyPassword(req.Username, req.Password)
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeUnauth, "用户名或密码错误"))
		return
	}

	pair, err := m.svc.GenerateTokenPair(user.ID, user.Username)
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, "token 签发失败"))
		return
	}

	m.publishEvent("user.login", user.ID, c.ClientIP())
	response.Success(c, pair)
}

func (m *Module) refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "缺少 refresh_token 参数")
		return
	}

	pair, err := m.svc.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeUnauth, err.Error()))
		return
	}

	response.Success(c, pair)
}

func (m *Module) authURL(p auth.SocialProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		state := fmt.Sprintf("%d", time.Now().UnixNano())
		url := p.AuthURL(state)
		response.Success(c, gin.H{
			"provider": p.Name(),
			"auth_url": url,
			"state":    state,
		})
	}
}

func (m *Module) callback(p auth.SocialProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Query("code")
		if code == "" {
			response.ParamErr(c, "缺少 code 参数")
			return
		}

		accessToken, err := p.Exchange(c.Request.Context(), code)
		if err != nil {
			response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
			return
		}

		userInfo, err := p.GetUserInfo(c.Request.Context(), accessToken)
		if err != nil {
			response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
			return
		}

		existing, _ := m.social.FindByProvider(p.Name(), userInfo.OpenID)
		if existing != nil {
			pair, err := m.svc.GenerateTokenPair(existing.UserID, userInfo.Nickname)
			if err != nil {
				response.Fail(c, appErrors.New(appErrors.CodeSystemErr, "token 签发失败"))
				return
			}
			response.Success(c, pair)
			return
		}

		user, err := m.user.CreateUser(auth.RegisterRequest{
			Username: fmt.Sprintf("%s_%s", p.Name(), userInfo.OpenID[:8]),
			Password: fmt.Sprintf("oauth_%s_%d", p.Name(), time.Now().Unix()),
			Nickname: userInfo.Nickname,
		})
		if err != nil {
			response.Fail(c, appErrors.New(appErrors.CodeSystemErr, "创建用户失败"))
			return
		}

		_ = m.social.Bind(c.Request.Context(), user.ID, p.Name(), userInfo)
		m.rbac.AssignRole(user.ID, string(auth.RoleUser))

		pair, err := m.svc.GenerateTokenPair(user.ID, user.Username)
		if err != nil {
			response.Fail(c, appErrors.New(appErrors.CodeSystemErr, "token 签发失败"))
			return
		}

		m.publishEvent("user.registered", user.ID, c.ClientIP())
		response.Success(c, pair)
	}
}

func (m *Module) bind(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := m.providers[name]
		if !ok {
			response.ParamErr(c, fmt.Sprintf("不支持的登录方式: %s", name))
			return
		}

		var req struct {
			Code string `json:"code" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ParamErr(c, "缺少 code 参数")
			return
		}

		accessToken, err := p.Exchange(c.Request.Context(), req.Code)
		if err != nil {
			response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
			return
		}

		userInfo, err := p.GetUserInfo(c.Request.Context(), accessToken)
		if err != nil {
			response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
			return
		}

		userID := c.GetUint("user_id")
		if err := m.social.Bind(c.Request.Context(), userID, name, userInfo); err != nil {
			response.Fail(c, appErrors.New(appErrors.CodeConflict, err.Error()))
			return
		}

		response.Success(c, gin.H{"message": fmt.Sprintf("已绑定 %s 账号", name)})
	}
}

func (m *Module) unbind(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("user_id")
		if err := m.social.Unbind(c.Request.Context(), userID, name); err != nil {
			response.Fail(c, appErrors.New(appErrors.CodeNotFound, err.Error()))
			return
		}
		response.Success(c, gin.H{"message": fmt.Sprintf("已解绑 %s 账号", name)})
	}
}

func (m *Module) logout(c *gin.Context) {
	userID := c.GetUint("user_id")

	header := c.GetHeader("Authorization")
	if len(header) > 7 && (header[:7] == "Bearer " || header[:7] == "bearer ") {
		tokenStr := header[7:]
		if claims, err := m.svc.ValidateAccessToken(tokenStr); err == nil {
			_ = m.blacklist.Revoke(context.Background(), claims.ID, claims.ExpiresAt.Time)
		}
	}

	m.publishEvent("user.logout", userID, c.ClientIP())
	response.Success(c, gin.H{"message": "已登出"})
}

func (m *Module) me(c *gin.Context) {
	userID := c.GetUint("user_id")

	user, err := m.user.FindByID(userID)
	if err != nil {
		response.FailCode(c, appErrors.CodeNotFound)
		return
	}

	response.Success(c, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"nickname": user.Nickname,
		"roles":    m.rbac.GetRoles(userID),
	})
}

func (m *Module) listAccounts(c *gin.Context) {
	userID := c.GetUint("user_id")
	accounts, _ := m.social.ListByUser(userID)
	response.Success(c, accounts)
}

func (m *Module) assignRole(c *gin.Context) {
	var req struct {
		UserID uint   `json:"user_id" binding:"required"`
		Role   string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "请求格式错误: "+err.Error())
		return
	}

	if req.Role != string(auth.RoleAdmin) && req.Role != string(auth.RoleUser) {
		response.ParamErr(c, fmt.Sprintf("无效角色: %s", req.Role))
		return
	}

	m.rbac.AssignRole(req.UserID, req.Role)
	response.Success(c, gin.H{"message": "角色已分配"})
}

func (m *Module) publishEvent(topic string, userID uint, ip string) {
	if m.bus == nil {
		return
	}
	_ = m.bus.Publish(context.Background(), topic, eventbus.Event{
		Source: "auth",
		Payload: map[string]any{
			"user_id": userID,
			"ip":      ip,
			"time":    time.Now().Format(time.RFC3339),
		},
	})
}

func (m *Module) AuthMiddleware() gin.HandlerFunc {
	return middleware.AuthRequiredWithBlacklist(m.svc, m.blacklist)
}

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
