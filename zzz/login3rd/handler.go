package login3rd

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	appErr "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Handler HTTP API 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建 Handler
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ---------- 平台 ----------

// ListPlatforms 列出所有支持的第三方平台
// @Summary      列出所有支持的第三方平台
// @Tags         login3rd
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.Response{data=[]Platform}
// @Router       /login3rd/platforms [get]
func (h *Handler) ListPlatforms(c *gin.Context) {
	platforms, err := h.svc.ListPlatforms(c.Request.Context())
	if err != nil {
		h.handleError(c, err)
		return
	}
	if platforms == nil {
		platforms = []Platform{}
	}
	response.Success(c, platforms)
}

// ---------- 账号 CRUD ----------

// CreateAccount 新增第三方账号
// @Summary      新增第三方账号
// @Tags         login3rd
// @Accept       json
// @Produce      json
// @Param        body  body  CreateAccountReq  true  "账号信息"
// @Success      200  {object}  response.Response{data=Account}
// @Router       /login3rd/accounts [post]
func (h *Handler) CreateAccount(c *gin.Context) {
	var req CreateAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	account, err := h.svc.CreateAccount(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, account)
}

// ListAccounts 获取第三方账号列表
// @Summary      获取第三方账号列表
// @Tags         login3rd
// @Accept       json
// @Produce      json
// @Param        page        query  int     false  "页码，默认 1"
// @Param        page_size   query  int     false  "每页数量，默认 20，最大 100"
// @Param        platform_id query  string  false  "筛选平台：jd / taobao / pdd"
// @Param        status      query  string  false  "筛选状态：active / expired / disabled"
// @Param        keyword     query  string  false  "搜索关键词（标签/用户名）"
// @Success      200  {object}  response.Response{data=PaginatedResp[Account]}
// @Router       /login3rd/accounts [get]
func (h *Handler) ListAccounts(c *gin.Context) {
	var req ListAccountsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误")
		return
	}
	req.Normalize()
	items, hasMore, total, err := h.svc.ListAccounts(c.Request.Context(), &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if items == nil {
		items = []Account{}
	}
	response.Success(c, PaginatedResp[Account]{Items: items, HasMore: hasMore, Total: total})
}

// GetAccount 获取第三方账号详情
// @Summary      获取第三方账号详情
// @Tags         login3rd
// @Accept       json
// @Produce      json
// @Param        id   path  int  true  "账号 ID"
// @Success      200  {object}  response.Response{data=Account}
// @Router       /login3rd/accounts/{id} [get]
func (h *Handler) GetAccount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的账号 ID")
		return
	}
	account, err := h.svc.GetAccount(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, account)
}

// UpdateAccount 更新第三方账号
// @Summary      更新第三方账号
// @Tags         login3rd
// @Accept       json
// @Produce      json
// @Param        id    path  int              true  "账号 ID"
// @Param        body  body  UpdateAccountReq  true  "更新字段"
// @Success      200  {object}  response.Response{data=Account}
// @Router       /login3rd/accounts/{id} [put]
func (h *Handler) UpdateAccount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的账号 ID")
		return
	}
	var req UpdateAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	account, err := h.svc.UpdateAccount(c.Request.Context(), id, &req)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, account)
}

// DeleteAccount 删除第三方账号
// @Summary      删除第三方账号
// @Tags         login3rd
// @Accept       json
// @Produce      json
// @Param        id   path  int  true  "账号 ID"
// @Success      200  {object}  response.Response
// @Router       /login3rd/accounts/{id} [delete]
func (h *Handler) DeleteAccount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的账号 ID")
		return
	}
	if err := h.svc.DeleteAccount(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// ---------- 登录操作 ----------

// AutoLogin 自动登录（使用保存的账号密码）
// @Summary      自动登录（使用保存的账号密码）
// @Tags         login3rd
// @Accept       json
// @Produce      json
// @Param        id   path  int  true  "账号 ID"
// @Success      200  {object}  response.Response{data=Account}
// @Router       /login3rd/accounts/{id}/login [post]
func (h *Handler) AutoLogin(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的账号 ID")
		return
	}
	account, err := h.svc.AutoLogin(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, account)
}

// CreateRemoteSession 创建远程浏览器登录会话
// @Summary      创建远程浏览器登录会话（返回链接，供 webhook 使用）
// @Tags         login3rd
// @Accept       json
// @Produce      json
// @Param        id   path  int  true  "账号 ID"
// @Success      200  {object}  response.Response{data=object{token=string,url=string}}
// @Router       /login3rd/accounts/{id}/remote-login [post]
func (h *Handler) CreateRemoteSession(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的账号 ID")
		return
	}
	token, err := h.svc.CreateRemoteSession(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	remoteURL := scheme + "://" + c.Request.Host + "/api/v1/login3rd/remote/session/" + token
	response.Success(c, gin.H{"token": token, "url": remoteURL})
}

// GetAccountStatus 获取账号登录状态
// @Summary      获取账号登录状态
// @Tags         login3rd
// @Accept       json
// @Produce      json
// @Param        id   path  int  true  "账号 ID"
// @Success      200  {object}  response.Response{data=AccountStatusResp}
// @Router       /login3rd/accounts/{id}/status [get]
func (h *Handler) GetAccountStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的账号 ID")
		return
	}
	status, err := h.svc.GetAccountStatus(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, status)
}

// RefreshSession 刷新登录会话
// @Summary      刷新登录会话（尝试用 refresh token 续期）
// @Tags         login3rd
// @Accept       json
// @Produce      json
// @Param        id   path  int  true  "账号 ID"
// @Success      200  {object}  response.Response{data=Account}
// @Router       /login3rd/accounts/{id}/refresh [post]
func (h *Handler) RefreshSession(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的账号 ID")
		return
	}
	account, err := h.svc.RefreshSession(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, account)
}

// VerifySession 验证登录会话
// @Summary      验证登录会话是否有效
// @Tags         login3rd
// @Accept       json
// @Produce      json
// @Param        id   path  int  true  "账号 ID"
// @Success      200  {object}  response.Response{data=object{valid=bool}}
// @Router       /login3rd/accounts/{id}/verify [post]
func (h *Handler) VerifySession(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的账号 ID")
		return
	}
	valid, err := h.svc.VerifySession(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"valid": valid})
}

// Logout 登出（清除 session 数据）
// @Summary      登出（清除 session 数据）
// @Tags         login3rd
// @Accept       json
// @Produce      json
// @Param        id   path  int  true  "账号 ID"
// @Success      200  {object}  response.Response
// @Router       /login3rd/accounts/{id}/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的账号 ID")
		return
	}
	if err := h.svc.Logout(c.Request.Context(), id); err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{})
}

// ---------- 远程浏览器 ----------

// RemoteSessionPage 远程浏览器页面
// @Summary      远程浏览器页面（Web 前端）
// @Tags         login3rd
// @Produce      html
// @Param        token   path  string  true  "会话 token"
// @Router       /login3rd/remote/session/{token} [get]
func (h *Handler) RemoteSessionPage(c *gin.Context) {
	token := c.Param("token")
	if _, err := h.svc.GetRemoteManager().GetSession(token); err != nil {
		response.Fail(c, appErr.New(appErr.CodeNotFound, "会话不存在或已过期"))
		return
	}
	html := buildRemoteHTML(token)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// RemoteSessionWS 远程浏览器 WebSocket 连接
// @Summary      远程浏览器 WebSocket 连接（推送截图 + 接收指令）
// @Tags         login3rd
// @Param        token   path  string  true  "会话 token"
// @Router       /login3rd/remote/session/{token}/ws [get]
func (h *Handler) RemoteSessionWS(c *gin.Context) {
	token := c.Param("token")
	session, err := h.svc.GetRemoteManager().GetSession(token)
	if err != nil {
		response.Fail(c, appErr.New(appErr.CodeNotFound, "会话不存在或已过期"))
		return
	}

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error().Err(err).Msg("升级 WebSocket 失败")
		return
	}
	defer conn.Close()

	// 发送初始截图
	if screen := session.GetScreenshotBase64(); screen != "" {
		conn.WriteJSON(gin.H{"type": "screenshot", "data": screen})
	}

	// 推送截图 + 接收用户指令循环
	done := false
	for !done {
		select {
		case <-session.Done:
			done = true
			result := session.Result
			if result != nil && result.Success {
				conn.WriteJSON(gin.H{"type": "login_success", "message": "登录成功！"})
			} else if result != nil && result.Error != "" {
				conn.WriteJSON(gin.H{"type": "login_failed", "error": result.Error})
			}
			return
		default:
			// 推截图
			if screen := session.GetScreenshotBase64(); screen != "" {
				conn.WriteJSON(gin.H{"type": "screenshot", "data": screen})
			}

			// 读用户指令（非阻塞带超时）
			conn.SetReadLimit(4096)
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var input struct {
				Type     string `json:"type"`
				Selector string `json:"selector,omitempty"`
				Value    string `json:"value,omitempty"`
			}
			if json.Unmarshal(msg, &input) != nil {
				continue
			}
			switch input.Type {
			case "input":
				if input.Selector != "" {
					chromedp.Run(session.BrowserCtx,
						chromedp.SendKeys(input.Selector, input.Value, chromedp.ByQuery),
					)
				}
			case "click":
				if input.Selector != "" {
					chromedp.Run(session.BrowserCtx,
						chromedp.Click(input.Selector, chromedp.ByQuery),
					)
				}
			case "close":
				done = true
			}
		}
	}
}

// ---------- 错误处理 ----------

func (h *Handler) handleError(c *gin.Context, err error) {
	var lErr *Login3rdError
	if errors.As(err, &lErr) {
		logger.Error().Err(err).Int("login3rd_code", lErr.Code).Msg("login3rd 业务错误")
		response.Fail(c, appErr.New(lErr.Code, lErr.Message))
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		response.Fail(c, appErr.New(appErr.CodeNotFound, "资源不存在"))
		return
	}
	logger.Error().Err(err).Msg("login3rd 服务内部错误")
	response.Fail(c, appErr.New(appErr.CodeSystemErr, "服务内部错误"))
}

// buildRemoteHTML 构建远程浏览器页面 HTML
func buildRemoteHTML(token string) string {
	html := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>远程浏览器登录 - Login3rd</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:#1a1a2e;color:#eee;display:flex;justify-content:center;min-height:100vh;padding:20px}
.container{max-width:800px;width:100%}
h1{font-size:20px;margin-bottom:16px;color:#e94560}
.status-bar{background:#16213e;border-radius:8px;padding:12px 16px;margin-bottom:16px;font-size:14px;display:flex;align-items:center;gap:8px}
.status-bar .dot{width:8px;height:8px;border-radius:50%;background:#0f3460;display:inline-block}
.status-bar .dot.connected{background:#00ff88}
.status-bar .dot.error{background:#e94560}
.screenshot-container{background:#16213e;border-radius:8px;overflow:hidden;margin-bottom:16px}
.screenshot-container img{width:100%;display:block}
.controls{background:#16213e;border-radius:8px;padding:16px}
.controls h3{font-size:14px;margin-bottom:12px;color:#aaa}
.input-group{display:flex;gap:8px;margin-bottom:8px}
.input-group input{flex:1;padding:10px 12px;border:1px solid #333;border-radius:6px;background:#0f3460;color:#eee;font-size:14px;outline:none}
.input-group input:focus{border-color:#e94560}
.input-group button{padding:10px 20px;border:none;border-radius:6px;background:#e94560;color:#fff;font-size:14px;cursor:pointer;white-space:nowrap}
.input-group button:hover{background:#c73650}
.input-group button.secondary{background:#0f3460}
.input-group button.secondary:hover{background:#1a4a7a}
.login-overlay{position:fixed;inset:0;background:rgba(0,0,0,0.8);display:flex;align-items:center;justify-content:center;z-index:100}
.login-overlay .box{background:#16213e;border-radius:12px;padding:40px;text-align:center;max-width:400px}
.login-overlay .box .icon{font-size:48px;margin-bottom:16px}
.login-overlay .box h2{font-size:24px;margin-bottom:8px}
.login-overlay .box p{color:#aaa;font-size:14px}
.hidden{display:none}
</style>
</head>
<body>
<div class="container">
<h1>🔑 远程浏览器登录</h1>
<div class="status-bar"><span class="dot" id="statusDot"></span><span id="statusText">正在连接...</span></div>
<div class="screenshot-container"><img id="screenshot" src="" alt="浏览器截图"></div>
<div class="controls">
<h3>操作面板</h3>
<div class="input-group"><input type="text" id="selectorInput" placeholder="CSS 选择器，如 #username"><input type="text" id="valueInput" placeholder="输入内容"><button onclick="sendInput()">发送</button></div>
<div class="input-group"><button class="secondary" onclick="sendClick('#captcha_img')">点击验证码</button><button class="secondary" onclick="sendClick('#loginBtn')">点击登录按钮</button></div>
</div>
</div>
<div id="loginOverlay" class="login-overlay hidden"><div class="box"><div class="icon" id="resultIcon">✅</div><h2 id="resultTitle">登录成功</h2><p id="resultMessage">Cookie 已保存到数据库，你可关闭此页面。</p></div></div>
<script>
var token=` + "`" + token + "`" + `;
var wsProto=location.protocol==='https:'?'wss:':'ws:';
var wsURL=wsProto+'//'+location.host+'/api/v1/login3rd/remote/session/'+token+'/ws';
var ws=null,connected=false;
function connect(){ws=new WebSocket(wsURL);ws.onopen=function(){connected=true;document.getElementById('statusDot').className='dot connected';document.getElementById('statusText').textContent='已连接，等待浏览器加载...'};ws.onmessage=function(e){try{var msg=JSON.parse(e.data);switch(msg.type){case'screenshot':document.getElementById('screenshot').src='data:image/png;base64,'+msg.data;document.getElementById('statusText').textContent='浏览器已加载';break;case'login_success':showResult(true,'登录成功','Cookie 已保存到数据库，你可关闭此页面。');break;case'login_failed':showResult(false,'登录失败',msg.error||'请重试');break}}catch(ex){}};ws.onclose=function(){connected=false;document.getElementById('statusDot').className='dot error';document.getElementById('statusText').textContent='连接已断开';setTimeout(connect,3000)};ws.onerror=function(){document.getElementById('statusDot').className='dot error';document.getElementById('statusText').textContent='连接错误'}}
function sendInput(){var s=document.getElementById('selectorInput').value,v=document.getElementById('valueInput').value;if(!s||!v||!connected)return;ws.send(JSON.stringify({type:'input',selector:s,value:v}));document.getElementById('valueInput').value=''}
function sendClick(s){if(!s||!connected)return;ws.send(JSON.stringify({type:'click',selector:s}))}
function showResult(ok,t,m){document.getElementById('resultIcon').textContent=ok?'✅':'❌';document.getElementById('resultTitle').textContent=t;document.getElementById('resultMessage').textContent=m;document.getElementById('loginOverlay').className='login-overlay';document.getElementById('statusText').textContent=t}
connect();
</script>
</body>
</html>`
	return strings.ReplaceAll(html, "`"+token+"`", token)
}
