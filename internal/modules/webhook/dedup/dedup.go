// Package dedup Webhook 内容去重器，基于 Redis SetNX 实现
package dedup

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Deduper 内容去重器
type Deduper struct {
	client *redis.Client
}

// New 创建去重器，client 为 nil 时返回 nil（去重功能禁用）
func New(client *redis.Client) *Deduper {
	if client == nil {
		return nil
	}
	return &Deduper{client: client}
}

// IsDuplicate 检查内容是否重复：在 windowSec 窗口期内发生过相同内容哈希则视为重复
// Redis SetNX 成功（首次出现）返回 false，键已存在返回 true
func (d *Deduper) IsDuplicate(ctx context.Context, webhookID, contentHash string, windowSec int) (bool, error) {
	if d == nil {
		return false, nil
	}
	key := fmt.Sprintf("webhook:dedup:%s:%s", webhookID, contentHash)
	existed, err := d.client.SetNX(ctx, key, 1, time.Duration(windowSec)*time.Second).Result()
	if err != nil {
		return false, err
	}
	return !existed, nil
}
