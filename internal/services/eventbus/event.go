package eventbus

import (
	"context"
	"time"
)

type Event struct {
	ID        string         `json:"id"`
	Topic     string         `json:"topic"`
	Source    string         `json:"source"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   map[string]any `json:"payload"`
}

type Subscription struct {
	ID      string
	Topic   string
	Handler EventHandler
}

type EventHandler func(ctx context.Context, event Event) error
