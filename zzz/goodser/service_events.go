package goodser

import (
	"context"

	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/services/outbox"
)

// 业务事件主题。外部系统（n8n / OpenClaw / LangBot）按主题订阅。
const (
	EventOrderCreated     = "goodser.order.created"
	EventOrderConfirmed   = "goodser.order.confirmed"
	EventOrderCancelled   = "goodser.order.cancelled"
	EventReserveConverted = "goodser.reserve.converted"
	EventReserveCancelled = "goodser.reserve.cancelled"
	EventReturnCreated    = "goodser.return.created"
	EventReturnConfirmed  = "goodser.return.confirmed"
)

// emitEvent 尽力写入外盒事件，失败只记录日志，不影响业务事务结果。
func (s *Service) emitEvent(ctx context.Context, topic, aggregateType, aggregateID string, payload any) {
	if s.events == nil {
		return
	}
	if _, err := s.events.Emit(ctx, outbox.EmitInput{
		Topic:         topic,
		Source:        "goodser",
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		Payload:       payload,
	}); err != nil {
		logger.Error().Err(err).Str("topic", topic).Str("aggregate_id", aggregateID).
			Msg("goodser: 写入外盒事件失败")
	}
}
