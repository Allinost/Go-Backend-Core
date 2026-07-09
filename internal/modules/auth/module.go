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
	apiKeys   *auth.ApiKeyService
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

	var apikeyStore auth.ApiKeyStore
	if database.DB != nil {
		if pool, ok := database.DB.MySQL["main"]; ok && pool.DB != nil {
			mysqlStore, err := auth.NewMySQLApiKeyStore(pool.DB)
			if err == nil {
				_ = mysqlStore.AutoMigrate()
				apikeyStore = mysqlStore
			}
		}
	}
	if apikeyStore == nil {
		apikeyStore = auth.NewInMemoryApiKeyStore()
	}
	m.apiKeys = auth.NewApiKeyService(apikeyStore)

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
	protected.PUT("/profile", m.updateProfile)
	protected.PUT("/password", m.changePassword)
	protected.POST("/logout", m.logout)
	protected.GET("/accounts", m.listAccounts)

	for name := range m.providers {
		n := name
		protected.POST("/"+n+"/bind", m.bind(n))
		protected.POST("/"+n+"/unbind", m.unbind(n))
	}

	admin := r.Group("")
	admin.Use(middleware.AuthRequiredWithBlacklist(m.svc, m.blacklist))
	admin.Use(middleware.RequirePermission(m.rbac, "user", "admin"))
	admin.GET("/users", m.listUsers)
	admin.GET("/users/:id", m.getUser)
	admin.PUT("/users/:id", m.updateUser)
	admin.DELETE("/users/:id", m.deleteUser)
	admin.POST("/roles/assign", m.assignRole)

	protected.POST("/api-keys", m.createAPIKey)
	protected.GET("/api-keys", m.listAPIKeys)
	protected.PUT("/api-keys/:id", m.updateAPIKey)
	protected.DELETE("/api-keys/:id", m.deleteAPIKey)
}

// register 用户注册
// @Summary      用户注册
// @Description  使用用户名/邮箱和密码注册新用户
// @Tags         auth-认证
// @Accept       json
// @Produce      json
// @Param        body  body  auth.RegisterRequest  true  "注册请求"
// @Success      200   {object}  response.Response{data=auth.TokenPair}
// @Failure      400   {object}  response.Response
// @Failure      409   {object}  response.Response
// @Router       /auth/register [post]
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

// login 用户登录
// @Summary      用户密码登录
// @Description  使用用户名/邮箱和密码登录，返回 JWT token 对
// @Tags         auth-认证
// @Accept       json
// @Produce      json
// @Param        body  body  auth.LoginRequest  true  "登录请求"
// @Success      200   {object}  response.Response{data=auth.TokenPair}
// @Failure      400   {object}  response.Response
// @Failure      401   {object}  response.Response
// @Router       /auth/login [post]
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

// refresh 刷新 Token
// @Summary      刷新访问令牌
// @Description  使用 refresh_token 获取新的 access_token
// @Tags         auth-认证
// @Accept       json
// @Produce      json
// @Param        body  body  object{refresh_token=string}  true  "刷新令牌"
// @Success      200   {object}  response.Response{data=auth.TokenPair}
// @Failure      401   {object}  response.Response
// @Router       /auth/refresh [post]
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

// authURL 获取第三方授权 URL
// @Summary      获取 OAuth 授权 URL
// @Description  获取指定第三方登录平台的授权跳转 URL
// @Tags         auth-社交登录
// @Produce      json
// @Param        provider  path  string  true  "平台名称: wechat/feishu/qq/apple/huawei/honor"
// @Success      200  {object}  response.Response{data=object{provider=string,auth_url=string,state=string}}
// @Router       /auth/{provider}/url [get]
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

// callback OAuth 回调处理
// @Summary      OAuth 登录回调
// @Description  第三方登录授权回调处理，自动注册或登录
// @Tags         auth-社交登录
// @Produce      json
// @Param        provider  path   string  true  "平台名称"
// @Param        code      query  string  true  "授权码"
// @Success      200  {object}  response.Response{data=auth.TokenPair}
// @Failure      400  {object}  response.Response
// @Router       /auth/{provider}/callback [post]
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

// logout 用户注销
// @Summary      用户注销
// @Description  将当前 JWT token 加入黑名单，立即失效
// @Tags         auth-认证
// @Produce      json
// @Success      200  {object}  response.Response
// @Failure      401  {object}  response.Response
// @Security     BearerAuth
// @Router       /auth/logout [post]
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

// me 获取当前用户信息
// @Summary      获取当前用户信息
// @Description  返回当前登录用户的详细信息
// @Tags         auth-用户
// @Produce      json
// @Success      200  {object}  response.Response
// @Failure      401  {object}  response.Response
// @Security     BearerAuth
// @Router       /auth/me [get]
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

func (m *Module) updateProfile(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req auth.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "请求格式错误: "+err.Error())
		return
	}

	user, err := m.user.UpdateUser(userID, req)
	if err != nil {
		response.FailCode(c, appErrors.CodeNotFound)
		return
	}

	response.Success(c, gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"nickname":   user.Nickname,
		"email":      user.Email,
		"avatar_url": user.AvatarURL,
		"phone":      user.Phone,
	})
}

func (m *Module) changePassword(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req auth.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "请求格式错误: "+err.Error())
		return
	}

	if err := m.user.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeUnauth, err.Error()))
		return
	}

	response.Success(c, gin.H{"message": "密码已修改"})
}

func (m *Module) listUsers(c *gin.Context) {
	page := 1
	pageSize := 20
	if p, err := parseInt(c.Query("page"), 1); err == nil {
		page = p
	}
	if ps, err := parseInt(c.Query("page_size"), 20); err == nil {
		pageSize = ps
	}
	search := c.Query("search")

	users, total, err := m.user.ListUsers(page, pageSize, search)
	if err != nil {
		response.FailCode(c, appErrors.CodeSystemErr)
		return
	}

	response.Success(c, gin.H{
		"data":  users,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}

func (m *Module) getUser(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		response.ParamErr(c, "无效的用户 ID")
		return
	}

	user, err := m.user.FindByID(id)
	if err != nil {
		response.FailCode(c, appErrors.CodeNotFound)
		return
	}

	response.Success(c, gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"nickname":   user.Nickname,
		"email":      user.Email,
		"avatar_url": user.AvatarURL,
		"phone":      user.Phone,
		"status":     user.Status,
		"created_at": user.CreatedAt,
		"updated_at": user.UpdatedAt,
		"roles":      m.rbac.GetRoles(user.ID),
	})
}

func (m *Module) updateUser(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		response.ParamErr(c, "无效的用户 ID")
		return
	}

	var req auth.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "请求格式错误: "+err.Error())
		return
	}

	user, err := m.user.UpdateUser(id, req)
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeNotFound, err.Error()))
		return
	}

	response.Success(c, gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"nickname":   user.Nickname,
		"email":      user.Email,
		"avatar_url": user.AvatarURL,
		"phone":      user.Phone,
		"status":     user.Status,
	})
}

func (m *Module) deleteUser(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		response.ParamErr(c, "无效的用户 ID")
		return
	}

	if err := m.user.DeleteUser(id); err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeNotFound, err.Error()))
		return
	}

	m.publishEvent("user.deleted", id, c.ClientIP())
	response.Success(c, gin.H{"message": "用户已删除"})
}

func parseInt(s string, defaultVal int) (int, error) {
	if s == "" {
		return defaultVal, nil
	}
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	if err != nil {
		return defaultVal, err
	}
	if v <= 0 {
		return defaultVal, nil
	}
	return v, nil
}

func parseUint(s string) (uint, error) {
	var v uint
	_, err := fmt.Sscanf(s, "%d", &v)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (m *Module) createAPIKey(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req struct {
		Name      string `json:"name" binding:"required"`
		Scopes    string `json:"scopes"`
		ExpiresIn string `json:"expires_in"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "请求格式错误: "+err.Error())
		return
	}

	scopes := req.Scopes
	if scopes == "" {
		scopes = "*:*"
	}

	var expiresAt *time.Time
	if req.ExpiresIn != "" {
		d, err := time.ParseDuration(req.ExpiresIn)
		if err == nil && d > 0 {
			t := time.Now().Add(d)
			expiresAt = &t
		}
	}

	key, err := m.apiKeys.GenerateKey(req.Name, userID, scopes, expiresAt)
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}

	response.Success(c, gin.H{
		"id":         key.ID,
		"name":       key.Name,
		"key_prefix": key.KeyPrefix,
		"raw_key":    key.RawKey,
		"scopes":     key.Scopes,
		"expires_at": key.ExpiresAt,
	})
}

func (m *Module) listAPIKeys(c *gin.Context) {
	userID := c.GetUint("user_id")
	isAdmin := m.rbac.HasPermission(userID, "user", "admin")

	if isAdmin {
		page, _ := parseInt(c.Query("page"), 1)
		pageSize, _ := parseInt(c.Query("page_size"), 20)
		keys, total, err := m.apiKeys.ListAll(page, pageSize)
		if err != nil {
			response.FailCode(c, appErrors.CodeSystemErr)
			return
		}
		response.Success(c, gin.H{
			"data":  keys,
			"total": total,
			"page":  page,
			"size":  pageSize,
		})
		return
	}

	keys, _ := m.apiKeys.ListByUser(userID)
	response.Success(c, keys)
}

func (m *Module) updateAPIKey(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		response.ParamErr(c, "无效的密钥 ID")
		return
	}

	userID := c.GetUint("user_id")
	isAdmin := m.rbac.HasPermission(userID, "user", "admin")

	if !isAdmin {
		keys, _ := m.apiKeys.ListByUser(userID)
		owns := false
		for _, k := range keys {
			if k.ID == id { owns = true; break }
		}
		if !owns {
			response.FailCode(c, appErrors.CodeForbidden)
			return
		}
	}

	var req struct {
		Name   *string `json:"name,omitempty"`
		Scopes *string `json:"scopes,omitempty"`
		Status *string `json:"status,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "请求格式错误: "+err.Error())
		return
	}

	updates := map[string]any{}
	if req.Name != nil { updates["name"] = *req.Name }
	if req.Scopes != nil { updates["scopes"] = *req.Scopes }
	if req.Status != nil { updates["status"] = *req.Status }

	if err := m.apiKeys.Update(id, updates); err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeNotFound, err.Error()))
		return
	}

	response.Success(c, gin.H{"message": "密钥已更新"})
}

func (m *Module) deleteAPIKey(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		response.ParamErr(c, "无效的密钥 ID")
		return
	}

	userID := c.GetUint("user_id")
	isAdmin := m.rbac.HasPermission(userID, "user", "admin")

	if !isAdmin {
		keys, _ := m.apiKeys.ListByUser(userID)
		owns := false
		for _, k := range keys {
			if k.ID == id { owns = true; break }
		}
		if !owns {
			response.FailCode(c, appErrors.CodeForbidden)
			return
		}
	}

	if err := m.apiKeys.Delete(id); err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeNotFound, err.Error()))
		return
	}

	response.Success(c, gin.H{"message": "密钥已删除"})
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
