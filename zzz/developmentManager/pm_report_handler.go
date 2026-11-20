package developmentmanager

import (
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *Handler) GetDashboard(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.pm.Dashboard(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *Handler) GetBoard(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	columns, err := h.pm.Board(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"columns": columns})
}

func (h *Handler) GetRoadmap(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	items, err := h.pm.Roadmap(c.Request.Context(), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *Handler) GetWeeklyReport(c *gin.Context) {
	if !h.pmReady(c) {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	now := time.Now()
	from := now.AddDate(0, 0, -7)
	to := now
	if raw := c.Query("from"); raw != "" {
		if t, err := parseReportTime(raw); err == nil {
			from = t
		} else {
			response.ParamErr(c, "from 格式错误")
			return
		}
	}
	if raw := c.Query("to"); raw != "" {
		if t, err := parseReportTime(raw); err == nil {
			to = t
		} else {
			response.ParamErr(c, "to 格式错误")
			return
		}
	}
	report, err := h.pm.WeeklyReport(c.Request.Context(), id, from, to)
	if err != nil {
		h.handleError(c, err)
		return
	}
	response.Success(c, report)
}

func parseReportTime(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", value)
}
