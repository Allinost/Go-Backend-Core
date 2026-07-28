package login3rd

import (
	"context"
	"encoding/json"

	"github.com/Allinost/go-backend-core/internal/services/scheduler"
)

var sessionCheckSvc *Service

type login3rdSessionCheckHandler struct{}

func (h *login3rdSessionCheckHandler) Name() string { return "login3rd_session_check" }

func (h *login3rdSessionCheckHandler) Execute(ctx context.Context, payload json.RawMessage) error {
	if sessionCheckSvc == nil {
		return nil
	}
	RunSessionCheck(ctx, sessionCheckSvc)
	return nil
}

func init() {
	scheduler.RegisterHandler(&login3rdSessionCheckHandler{})
}
