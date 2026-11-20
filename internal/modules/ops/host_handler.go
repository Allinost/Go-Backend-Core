package ops

import (
	"errors"
	"fmt"
	"strconv"

	appErrors "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/Allinost/go-backend-core/internal/services/audit"
	"github.com/gin-gonic/gin"
)

type HostHandler struct {
	svc *HostService
}

func NewHostHandler(svc *HostService) *HostHandler {
	return &HostHandler{svc: svc}
}

func (h *HostHandler) List(c *gin.Context) {
	var req ListHostsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	items, hasMore, total, err := h.svc.List(c.Request.Context(), &req)
	if err != nil {
		hostFail(c, err)
		return
	}
	if items == nil {
		items = []Host{}
	}
	response.Success(c, gin.H{"items": items, "has_more": hasMore, "total": total})
}

func (h *HostHandler) Create(c *gin.Context) {
	var req CreateHostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.Create(c.Request.Context(), &req, hostActor(c))
	if err != nil {
		hostFail(c, err)
		return
	}
	hostAudit(c, "create", item.Name)
	response.Success(c, item)
}

func (h *HostHandler) Get(c *gin.Context) {
	id, ok := hostID(c)
	if !ok {
		return
	}
	item, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		hostFail(c, err)
		return
	}
	response.Success(c, item)
}

func (h *HostHandler) Update(c *gin.Context) {
	id, ok := hostID(c)
	if !ok {
		return
	}
	var req UpdateHostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	item, err := h.svc.Update(c.Request.Context(), id, &req, hostActor(c))
	if err != nil {
		hostFail(c, err)
		return
	}
	hostAudit(c, "update", item.Name)
	response.Success(c, item)
}

func (h *HostHandler) Delete(c *gin.Context) {
	id, ok := hostID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id, hostActor(c)); err != nil {
		hostFail(c, err)
		return
	}
	hostAudit(c, "delete", "host:"+c.Param("id"))
	response.Success(c, gin.H{"deleted": true})
}

func (h *HostHandler) Test(c *gin.Context) {
	id, ok := hostID(c)
	if !ok {
		return
	}
	result, err := h.svc.Test(c.Request.Context(), id)
	if err != nil {
		hostFail(c, err)
		return
	}
	hostAudit(c, "connect", "host:"+c.Param("id")+" online="+fmt.Sprint(result.Online))
	response.Success(c, result)
}

func (h *HostHandler) Status(c *gin.Context) {
	id, ok := hostID(c)
	if !ok {
		return
	}
	result, err := h.svc.Status(c.Request.Context(), id)
	if err != nil {
		hostFail(c, err)
		return
	}
	response.Success(c, result)
}

func hostID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ParamErr(c, "无效的主机 ID")
		return 0, false
	}
	return id, true
}

func hostActor(c *gin.Context) string {
	if v, ok := c.Get("username"); ok {
		return fmt.Sprint(v)
	}
	return ""
}

func hostFail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrHostNotFound):
		response.Fail(c, appErrors.New(appErrors.CodeNotFound, err.Error()))
	case errors.Is(err, ErrHostExists):
		response.Fail(c, appErrors.New(appErrors.CodeConflict, err.Error()))
	case errors.Is(err, ErrHostInvalid), errors.Is(err, ErrHostCredential):
		response.Fail(c, appErrors.New(appErrors.CodeParamErr, err.Error()))
	default:
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
	}
}

func hostAudit(c *gin.Context, action, detail string) {
	uid := ""
	if v, ok := c.Get("user_id"); ok {
		uid = fmt.Sprint(v)
	}
	var err error
	switch action {
	case "create":
		err = audit.RecordCreate(c.Request.Context(), uid, hostActor(c), "ops.host", detail)
	case "delete":
		err = audit.RecordDelete(c.Request.Context(), uid, hostActor(c), "ops.host", detail)
	default:
		err = audit.RecordUpdate(c.Request.Context(), uid, hostActor(c), "ops.host", detail)
	}
	if err != nil {
		logger.Warn().Err(err).Msg("ops: 主机审计记录失败")
	}
}
