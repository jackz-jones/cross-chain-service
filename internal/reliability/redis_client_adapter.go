package reliability

import (
	"context"
	"time"

	commonEvent "github.com/jackz-jones/common/event"
	"github.com/redis/go-redis/v9"
)

// RedisNodeAdapter 适配 common/event.RedisNode 到 RedisClient 和 RedisHashClient 接口
// 同时实现两个接口，复用同一个底层连接
type RedisNodeAdapter struct {
	node commonEvent.RedisNode
}

// NewRedisAdapterFromCommon 基于 common/event.RedisClient 创建统一的 Redis 适配器
// 同时实现 RedisClient 和 RedisHashClient 接口，复用 common 包已有的 Redis 连接管理能力
func NewRedisAdapterFromCommon(client *commonEvent.RedisClient) *RedisNodeAdapter {
	return &RedisNodeAdapter{node: client.RedisClient}
}

// SetNX 实现 RedisClient.SetNX
func (a *RedisNodeAdapter) SetNX(
	ctx context.Context, key string, value interface{}, expiration time.Duration,
) (bool, error) {
	return a.node.SetNX(ctx, key, value, expiration).Result()
}

// Exists 实现 RedisClient.Exists
func (a *RedisNodeAdapter) Exists(ctx context.Context, key string) (bool, error) {
	n, err := a.node.Exists(ctx, key).Result()
	return n > 0, err
}

// HSet 实现 RedisHashClient.HSet
func (a *RedisNodeAdapter) HSet(ctx context.Context, key string, field string, value interface{}) error {
	return a.node.HSet(ctx, key, field, value).Err()
}

// HGet 实现 RedisHashClient.HGet
func (a *RedisNodeAdapter) HGet(ctx context.Context, key string, field string) (string, error) {
	result, err := a.node.HGet(ctx, key, field).Result()
	if err == redis.Nil {
		return "", nil
	}
	return result, err
}

// HGetAll 实现 RedisHashClient.HGetAll
func (a *RedisNodeAdapter) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return a.node.HGetAll(ctx, key).Result()
}

// Expire 实现 RedisHashClient.Expire
func (a *RedisNodeAdapter) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return a.node.Expire(ctx, key, ttl).Err()
}

// Set 实现带 TTL 的 SET（一次 RTT，等价 SETEX）
// ttl <= 0 时表示不设置过期时间
func (a *RedisNodeAdapter) Set(
	ctx context.Context, key string, value interface{}, ttl time.Duration,
) error {
	return a.node.Set(ctx, key, value, ttl).Err()
}

// Get 获取字符串值；key 不存在时返回 ("", nil)
func (a *RedisNodeAdapter) Get(ctx context.Context, key string) (string, error) {
	result, err := a.node.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return result, err
}

// Del 删除单个 key
func (a *RedisNodeAdapter) Del(ctx context.Context, key string) error {
	return a.node.Del(ctx, key).Err()
}

// SAdd 将 member 加入到 key 对应的集合中
func (a *RedisNodeAdapter) SAdd(ctx context.Context, key string, member interface{}) error {
	return a.node.SAdd(ctx, key, member).Err()
}

// SMembers 返回集合中所有成员
func (a *RedisNodeAdapter) SMembers(ctx context.Context, key string) ([]string, error) {
	return a.node.SMembers(ctx, key).Result()
}

// SRem 从集合中移除 member
func (a *RedisNodeAdapter) SRem(ctx context.Context, key string, member interface{}) error {
	return a.node.SRem(ctx, key, member).Err()
}