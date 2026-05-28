package reliability

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// goRedisClient 适配 go-redis/v9 到 RedisClient 接口
type goRedisClient struct {
	client *redis.Client
}

// NewGoRedisClient 创建基于 go-redis/v9 的 RedisClient 实现
// 参数与 SubscribeConf 配置一致
func NewGoRedisClient(confType, addr, username, password, masterName string) RedisClient {
	// 哨兵模式
	if confType == "sentinel" && masterName != "" {
		failoverOpts := &redis.FailoverOptions{
			MasterName:    masterName,
			SentinelAddrs: []string{addr},
			Username:      username,
			Password:      password,
		}
		client := redis.NewFailoverClient(failoverOpts)
		return &goRedisClient{client: client}
	}

	// 集群模式
	if confType == "cluster" {
		clusterClient := redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    []string{addr},
			Username: username,
			Password: password,
		})
		return &goRedisClusterClient{client: clusterClient}
	}

	// 单机模式
	opts := &redis.Options{
		Addr:     addr,
		Username: username,
		Password: password,
	}
	client := redis.NewClient(opts)
	return &goRedisClient{client: client}
}

// SetNX 实现 RedisClient.SetNX
func (c *goRedisClient) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	return c.client.SetNX(ctx, key, value, expiration).Result()
}

// Exists 实现 RedisClient.Exists
func (c *goRedisClient) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, key).Result()
	return n > 0, err
}

// goRedisClusterClient 适配集群模式
type goRedisClusterClient struct {
	client *redis.ClusterClient
}

// SetNX 实现 RedisClient.SetNX
func (c *goRedisClusterClient) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	return c.client.SetNX(ctx, key, value, expiration).Result()
}

// Exists 实现 RedisClient.Exists
func (c *goRedisClusterClient) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.clusterExists(ctx, key)
	return n > 0, err
}

// clusterExists 使用 Lua 脚本在集群模式下执行 EXISTS 命令
func (c *goRedisClusterClient) clusterExists(ctx context.Context, key string) (int64, error) {
	script := redis.NewScript("return redis.call('exists',KEYS[1])")
	return script.Run(ctx, c.client, []string{key}).Int64()
}

// goRedisHashClient 适配 go-redis/v9 到 RedisHashClient 接口
type goRedisHashClient struct {
	client *redis.Client
}

// NewGoRedisHashClient 创建基于 go-redis/v9 的 RedisHashClient 实现
func NewGoRedisHashClient(confType, addr, username, password, masterName string) RedisHashClient {
	// 哨兵模式
	if confType == "sentinel" && masterName != "" {
		failoverOpts := &redis.FailoverOptions{
			MasterName:    masterName,
			SentinelAddrs: []string{addr},
			Username:      username,
			Password:      password,
		}
		client := redis.NewFailoverClient(failoverOpts)
		return &goRedisHashClient{client: client}
	}

	// 单机模式
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Username: username,
		Password: password,
	})

	return &goRedisHashClient{client: client}
}

// HSet 实现 RedisHashClient.HSet
func (c *goRedisHashClient) HSet(ctx context.Context, key string, field string, value interface{}) error {
	return c.client.HSet(ctx, key, field, value).Err()
}

// HGet 实现 RedisHashClient.HGet
func (c *goRedisHashClient) HGet(ctx context.Context, key string, field string) (string, error) {
	return c.client.HGet(ctx, key, field).Result()
}

// HGetAll 实现 RedisHashClient.HGetAll
func (c *goRedisHashClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.client.HGetAll(ctx, key).Result()
}

// Expire 实现 RedisHashClient.Expire
func (c *goRedisHashClient) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.client.Expire(ctx, key, ttl).Err()
}