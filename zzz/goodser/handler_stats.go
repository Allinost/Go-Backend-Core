package goodser

import (
	"strconv"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// GetInventoryStatsREST 库存目录统计总览
// @Summary      库存目录统计总览
// @Description  汇总可用/锁定库存、库存价值、订单与退货数量、按系列分布及区间内出库/退货趋势；支持 days 或 start_date/end_date
// @Tags         zzz-goodser
// @Produce      json
// @Param        id          path   string  true   "库存目录 ID"
// @Param        days        query  int     false  "统计天数，默认 30，范围 1-365"
// @Param        start_date  query  string  false  "自定义开始日期 YYYY-MM-DD，需与 end_date 同时提供"
// @Param        end_date    query  string  false  "自定义结束日期 YYYY-MM-DD，含首尾，最长 366 天"
// @Success      200  {object}  response.Response{data=StatsOverview}
// @Router       /zzz-goodser/inventories/{id}/stats [get]
func (h *Handler) GetInventoryStatsREST(c *gin.Context) {
	startRaw := c.Query("start_date")
	endRaw := c.Query("end_date")
	if startRaw != "" || endRaw != "" {
		if startRaw == "" || endRaw == "" {
			response.ParamErr(c, "自定义区间需同时提供 start_date 与 end_date")
			return
		}
		start, err1 := time.ParseInLocation("2006-01-02", startRaw, time.Local)
		end, err2 := time.ParseInLocation("2006-01-02", endRaw, time.Local)
		if err1 != nil || err2 != nil {
			response.ParamErr(c, "日期格式应为 YYYY-MM-DD")
			return
		}
		stats, err := h.svc.GetStatsOverviewRange(c.Request.Context(), c.Param("id"), start, end)
		if err != nil {
			h.handleError(c, err)
			return
		}
		response.Success(c, stats)
		return
	}
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
