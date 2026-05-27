// Package reliability 可靠性保障机制
package reliability

import (
	"context"
	"fmt"
	"time"
)

// IdempotencyChecker 幂等性检查器接口
type IdempotencyChecker interface {
	// IsDuplicate 检查事件是否已处理过
	// eventKey: 事件唯一标识（通常为 OriginHash 或 TxId）
	// 返回 true 表示重复事件
	IsDuplicate(ctx context.Context, eventKey string) (bool, error)

	// MarkProcessed 标记事件已处理
	// eventKey: 事件唯一标识
	// ttl: 过期时间（防止无限增长）
	MarkProcessed(ctx context.Context, eventKey string, ttl time.Duration) error
}

// RedisIdempotencyChecker 基于 Redis 的幂等性检查器
type RedisIdempotencyChecker struct {
	client RedisClient
	prefix string
}

// RedisClient Redis 客户端接口（解耦具体实现）
type RedisClient interface {
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error)
	Exists(ctx context.Context, key string) (bool, error)
}

// NewRedisIdempotencyChecker 创建 Redis 幂等性检查器
func NewRedisIdempotencyChecker(client RedisClient, prefix string) *RedisIdempotencyChecker {
	if prefix == "" {
		prefix = "cross_chain:idempotent:"
	}
	return &RedisIdempotencyChecker{
		client: client,
		prefix: prefix,
	}
}

// IsDuplicate 检查事件是否已处理过
func (c *RedisIdempotencyChecker) IsDuplicate(ctx context.Context, eventKey string) (bool, error) {
	key := c.prefix + eventKey
	exists, err := c.client.Exists(ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to check idempotency: %v", err)
	}
	return exists, nil
}

// MarkProcessed 标记事件已处理
func (c *RedisIdempotencyChecker) MarkProcessed(ctx context.Context, eventKey string, ttl time.Duration) error {
	key := c.prefix + eventKey
	ok, err := c.client.SetNX(ctx, key, "1", ttl)
	if err != nil {
		return fmt.Errorf("failed to mark event as processed: %v", err)
	}
	if !ok {
		// 已存在，说明是重复事件
		return nil
	}
	return nil
}

// InMemoryIdempotencyChecker 内存版幂等性检查器（用于测试）
type InMemoryIdempotencyChecker struct {
	processed map[string]time.Time
}

// NewInMemoryIdempotencyChecker 创建内存版幂等性检查器
func NewInMemoryIdempotencyChecker() *InMemoryIdempotencyChecker {
	return &InMemoryIdempotencyChecker{
		processed: make(map[string]time.Time),
	}
}

// IsDuplicate 检查事件是否已处理过
func (c *InMemoryIdempotencyChecker) IsDuplicate(_ context.Context, eventKey string) (bool, error) {
	expireAt, exists := c.processed[eventKey]
	if !exists {
		return false, nil
	}
	// 检查是否过期
	if time.Now().After(expireAt) {
		delete(c.processed, eventKey)
		return false, nil
	}
	return true, nil
}

// MarkProcessed 标记事件已处理
func (c *InMemoryIdempotencyChecker) MarkProcessed(_ context.Context, eventKey string, ttl time.Duration) error {
	c.processed[eventKey] = time.Now().Add(ttl)
	return nil
}
