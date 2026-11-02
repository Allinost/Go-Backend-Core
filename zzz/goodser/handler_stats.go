package goodser

import (
	"strconv"

	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// GetInventoryStatsREST 库存目录统计总览
// @Summary      库存目录统计总览
// @Description  汇总可用/锁定库存、库存价值、订单与退货数量、按系列分布及最近 N 天出库/退货趋势
// @Tags         zzz-goodser
// @Produce      json
// @Param        id    path   string  true   "库存目录 ID"
// @Param        days  query  int     false  "统计天数，默认 30，范围 1-365"
// @Success      200  {object}  response.Response{data=StatsOverview}
// @Router       /zzz-goodser/inventories/{id}/stats [get]
func (h *Handler) GetInventoryStatsREST(c *gin.Context) {
	days := 0
	if raw := c.Query("days"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 365 {
			response.ParamErr(c, "days 参数需在 1-365 之间")
			return
		}
		days = value
	}
	stats, err := h.svc.GetStatsOverview(c.Request.Context(), c.Param("id"), days)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, stats)
}
