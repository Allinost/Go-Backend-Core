package settings

import (
	"fmt"
	"strconv"

	appErrors "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/Allinost/go-backend-core/internal/services/audit"
	"github.com/gin-gonic/gin"
)

type configUpdateReq struct {
	Values map[string]any `json:"values"`
	Note   string         `json:"note"`
}

type configResetReq struct {
	Keys []string `json:"keys"`
	Note string   `json:"note"`
}

func (h *Handler) configReady(c *gin.Context) bool {
	if h.configSvc == nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, "配置中心未初始化"))
		return false
	}
	return true
}

// ConfigSchema 返回全部配置项定义
// @Summary      配置 Schema
// @Tags         settings
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /settings/config/schema [get]
// @Security     BearerAuth
func (h *Handler) ConfigSchema(c *gin.Context) {
	if !h.configReady(c) {
		return
	}
	response.Success(c, gin.H{"specs": h.configSvc.Specs()})
}

// GetConfig 返回当前配置值（敏感项脱敏）
// @Summary      获取配置
// @Tags         settings
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /settings/config [get]
// @Security     BearerAuth
func (h *Handler) GetConfig(c *gin.Context) {
	if !h.configReady(c) {
		return
	}
	response.Success(c, gin.H{"values": h.configSvc.Values(true), "degraded": h.configSvc.Degraded()})
}

// UpdateConfig 更新配置（校验 + 持久化 + 热更新 + 审计）
// @Summary      更新配置
// @Tags         settings
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /settings/config [put]
// @Security     BearerAuth
func (h *Handler) UpdateConfig(c *gin.Context) {
	if !h.configReady(c) {
		return
	}
	var req configUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	if len(req.Values) == 0 {
		response.ParamErr(c, "values 不能为空")
		return
	}
	uid, username := actor(c)
	if err := h.configSvc.Update(c.Request.Context(), req.Values, uid, req.Note); err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeParamErr, err.Error()))
		return
	}
	if err := audit.RecordUpdate(c.Request.Context(), uid, username, "settings.config", "配置中心更新"); err != nil {
		logger.Warn().Err(err).Msg("settings: 配置审计记录失败")
	}
	response.Success(c, gin.H{"values": h.configSvc.Values(true)})
}

// ValidateConfig 只校验，不落库
// @Summary      校验配置
// @Tags         settings
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /settings/config/validate [post]
// @Security     BearerAuth
func (h *Handler) ValidateConfig(c *gin.Context) {
	if !h.configReady(c) {
		return
	}
	var req configUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	normalized, err := h.configSvc.ValidatePatch(req.Values)
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeParamErr, err.Error()))
		return
	}
	response.Success(c, gin.H{"values": normalized})
}

// ResetConfig 重置为默认值
// @Summary      重置配置
// @Tags         settings
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /settings/config/reset [post]
// @Security     BearerAuth
func (h *Handler) ResetConfig(c *gin.Context) {
	if !h.configReady(c) {
		return
	}
	var req configResetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	uid, username := actor(c)
	if err := h.configSvc.Reset(c.Request.Context(), req.Keys, uid, req.Note); err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}
	if err := audit.RecordUpdate(c.Request.Context(), uid, username, "settings.config", "配置中心重置"); err != nil {
		logger.Warn().Err(err).Msg("settings: 配置审计记录失败")
	}
	response.Success(c, gin.H{"values": h.configSvc.Values(true)})
}

// ConfigHistory 返回配置变更历史
// @Summary      配置历史
// @Tags         settings
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /settings/config/history [get]
// @Security     BearerAuth
func (h *Handler) ConfigHistory(c *gin.Context) {
	if !h.configReady(c) {
		return
	}
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	items, err := h.configSvc.History(c.Request.Context(), c.Query("key"), limit)
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}
	response.Success(c, gin.H{"items": items})
}

func actor(c *gin.Context) (string, string) {
	uid := ""
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(uint); ok {
			uid = strconv.FormatUint(uint64(id), 10)
		}
	}
	username, _ := c.Get("username")
	return uid, fmt.Sprint(username)
}
