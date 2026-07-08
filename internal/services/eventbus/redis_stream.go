package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	maxRetries  = 5
	baseBackoff = 500 * time.Millisecond
	maxBackoff  = 30 * time.Second
)

type RedisStreamBus struct {
	client    redis.UniversalClient
	groupName string
	consumer  string
}

func NewRedisStream(client redis.UniversalClient, groupName, consumer string) *RedisStreamBus {
	return &RedisStreamBus{
		client:    client,
		groupName: groupName,
		consumer:  consumer,
	}
}

func (b *RedisStreamBus) Publish(ctx context.Context, topic string, event Event) error {
	event.Topic = topic
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	return b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: topic,
		Values: map[string]any{
			"data":      string(data),
			"timestamp": event.Timestamp.UnixMilli(),
		},
		MaxLen: 10000,
		Approx: true,
	}).Err()
}

func (b *RedisStreamBus) PublishAsync(ctx context.Context, topic string, event Event) <-chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- b.Publish(ctx, topic, event)
		close(ch)
	}()
	return ch
}

func (b *RedisStreamBus) Subscribe(topic string, handler EventHandler) (Subscription, error) {
	return Subscription{}, fmt.Errorf("Subscribe is not supported directly on RedisStreamBus; use ConsumeGroup")
}

func (b *RedisStreamBus) Unsubscribe(sub Subscription) error {
	return fmt.Errorf("Unsubscribe is not supported directly on RedisStreamBus; manage consumer lifecycle externally")
}

func (b *RedisStreamBus) EnsureGroup(ctx context.Context, topics ...string) error {
	for _, topic := range topics {
		err := b.client.XGroupCreateMkStream(ctx, topic, b.groupName, "0").Err()
		if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
			return fmt.Errorf("create group %s for %s: %w", b.groupName, topic, err)
		}
	}
	return nil
}

func (b *RedisStreamBus) GroupInfo(ctx context.Context, topic string) (*redis.XInfoGroup, error) {
	groups, err := b.client.XInfoGroups(ctx, topic).Result()
	if err != nil {
		return nil, fmt.Errorf("xinfo groups: %w", err)
	}
	for _, g := range groups {
		if g.Name == b.groupName {
			return &g, nil
		}
	}
	return nil, fmt.Errorf("group %s not found on %s", b.groupName, topic)
}

func (b *RedisStreamBus) PendingCount(ctx context.Context, topic string) (int64, error) {
	pending, err := b.client.XPending(ctx, topic, b.groupName).Result()
	if err != nil {
		return 0, fmt.Errorf("xpending: %w", err)
	}
	return pending.Count, nil
}

func (b *RedisStreamBus) ConsumeFromID(ctx context.Context, topics []string, startID string, handler EventHandler) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		streams, err := b.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    b.groupName,
			Consumer: b.consumer,
			Streams:  append(topics, startID),
			Count:    10,
			Block:    2 * time.Second,
		}).Result()

		if err != nil {
			if err == redis.Nil {
				continue
			}
			return fmt.Errorf("xreadgroup: %w", err)
		}

		startID = ">"

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				event, err := decodeEvent(msg)
				if err != nil {
					continue
				}

				if err := handler(ctx, event); err != nil {
					continue
				}

				b.client.XAck(ctx, stream.Stream, b.groupName, msg.ID)
			}
		}
	}
}

func (b *RedisStreamBus) Consume(ctx context.Context, topics []string, handler EventHandler) error {
	var retries int

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		streams, err := b.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    b.groupName,
			Consumer: b.consumer,
			Streams:  append(topics, ">"),
			Count:    10,
			Block:    2 * time.Second,
		}).Result()

		if err != nil {
			if err == redis.Nil {
				continue
			}

			retries++
			if retries > maxRetries {
				return fmt.Errorf("xreadgroup: %w (after %d retries)", err, maxRetries)
			}

			backoff := time.Duration(math.Min(float64(baseBackoff)*math.Pow(2, float64(retries-1)), float64(maxBackoff)))
			log.Printf("[eventbus] Redis 断线重连 %d/%d, 等待 %v, 错误: %v", retries, maxRetries, backoff, err)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			continue
		}

		retries = 0

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				event, err := decodeEvent(msg)
				if err != nil {
					continue
				}

				if err := handler(ctx, event); err != nil {
					continue
				}

				b.client.XAck(ctx, stream.Stream, b.groupName, msg.ID)
			}
		}
	}
}

func decodeEvent(msg redis.XMessage) (Event, error) {
	var event Event
	dataStr, ok := msg.Values["data"].(string)
	if !ok {
		return event, fmt.Errorf("missing data field")
	}
	if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
		return event, fmt.Errorf("unmarshal event: %w", err)
	}
	return event, nil
}
