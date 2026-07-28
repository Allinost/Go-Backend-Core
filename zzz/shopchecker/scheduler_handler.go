package shopchecker

import (
	"context"
	"encoding/json"

	"github.com/Allinost/go-backend-core/internal/services/scheduler"
)

type shopcheckerPriceScanHandler struct{}

func (h *shopcheckerPriceScanHandler) Name() string { return "shopchecker_price_scan" }

func (h *shopcheckerPriceScanHandler) Execute(ctx context.Context, payload json.RawMessage) error {
	// 实际调度由独立的 shopchecker scheduler 驱动（1s 轮询）
	// 此处仅占位，使 centralized scheduler 中可查看此任务的管理记录
	return nil
}

func init() {
	scheduler.RegisterHandler(&shopcheckerPriceScanHandler{})
}
