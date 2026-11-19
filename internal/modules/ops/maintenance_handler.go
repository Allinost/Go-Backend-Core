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

type MaintenanceHandler struct {
	svc *MaintenanceService
}

func NewMaintenanceHandler(svc *MaintenanceService) *MaintenanceHandler {
	return &MaintenanceHandler{svc: svc}
}

func (h *MaintenanceHandler) Status(c *gin.Context) {
	status, err := h.svc.Status(c.Request.Context())
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}
	response.Success(c, status)
}

func (h *MaintenanceHandler) Start(c *gin.Context) {
	var req StartMaintenanceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	operator := maintenanceActor(c)
	item, err := h.svc.Start(c.Request.Context(), &req, operator)
	if err != nil {
		maintenanceFail(c, err)
		return
	}
	maintenanceAudit(c, "开始维护 mode="+item.Mode+" target="+item.TargetNode)
	response.Success(c, item)
}

func (h *MaintenanceHandler) End(c *gin.Context) {
	var req EndMaintenanceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "参数错误: "+err.Error())
		return
	}
	operator := maintenanceActor(c)
	item, err := h.svc.End(c.Request.Context(), &req, operator)
	if err != nil {
		maintenanceFail(c, err)
		return
	}
	maintenanceAudit(c, "结束维护 mode="+item.Mode+" switched_to_l2="+fmt.Sprint(item.SwitchedToL2))
	response.Success(c, item)
}

func (h *MaintenanceHandler) History(c *gin.Context) {
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	items, err := h.svc.History(c.Request.Context(), limit)
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}
	if items == nil {
		items = []Maintenance{}
	}
	response.Success(c, gin.H{"items": items})
}

func maintenanceFail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrMaintenanceActive):
		response.Fail(c, appErrors.New(appErrors.CodeConflict, err.Error()))
	case errors.Is(err, ErrMaintenanceNotFound):
		response.Fail(c, appErrors.New(appErrors.CodeNotFound, err.Error()))
	case errors.Is(err, ErrInvalidMaintenanceMode),
		errors.Is(err, ErrMaintenanceSyncRequired),
		errors.Is(err, ErrMaintenanceSyncBack):
		response.Fail(c, appErrors.New(appErrors.CodeParamErr, err.Error()))
	default:
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
	}
}

func maintenanceActor(c *gin.Context) string {
	if v, ok := c.Get("username"); ok {
		return fmt.Sprint(v)
	}
	return ""
}

func maintenanceAudit(c *gin.Context, detail string) {
	uid := ""
	if v, ok := c.Get("user_id"); ok {
		uid = fmt.Sprint(v)
	}
	if err := audit.RecordUpdate(c.Request.Context(), uid, maintenanceActor(c), "ops.maintenance", detail); err != nil {
		logger.Warn().Err(err).Msg("ops: 维护审计记录失败")
	}
}
