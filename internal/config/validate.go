// Package config 配置管理
package config

import (
	"fmt"
	"os"
	"strconv"
)

// chainInteractiveServiceKey 是核心依赖：跨链服务通过它拉取链/合约元数据。
const chainInteractiveServiceKey = "chain-interactive-service"

// Validate 校验配置有效性
func (c *Config) Validate() error {
	// 校验订阅配置
	if c.SubscribeConf.RedisAddr == "" {
		return fmt.Errorf("SubscribeConf.RedisAddr is required")
	}
	if c.SubscribeConf.GroupName == "" {
		return fmt.Errorf("SubscribeConf.GroupName is required")
	}

	// 校验外部 gRPC 配置
	if len(c.ExternalGrpcConfs) == 0 {
		return fmt.Errorf("at least one ExternalGrpcConf is required")
	}
	for name, conf := range c.ExternalGrpcConfs {
		if conf == nil {
			return fmt.Errorf("ExternalGrpcConfs[%s] is nil", name)
		}
		if conf.Endpoint == "" {
			return fmt.Errorf("ExternalGrpcConfs[%s].Endpoint is required", name)
		}
	}
	// 必须显式配置 chain-interactive-service，否则服务无法运行
	if _, ok := c.ExternalGrpcConfs[chainInteractiveServiceKey]; !ok {
		return fmt.Errorf("ExternalGrpcConfs[%q] is required", chainInteractiveServiceKey)
	}

	// 校验可靠性配置
	if c.ReliabilityConf.MaxRetries < 0 {
		return fmt.Errorf("ReliabilityConf.MaxRetries must be >= 0")
	}
	if c.ReliabilityConf.RetryMultiplier < 0 {
		return fmt.Errorf("ReliabilityConf.RetryMultiplier must be >= 0")
	}
	if c.ReliabilityConf.IdempotencyTTL < 0 {
		return fmt.Errorf("ReliabilityConf.IdempotencyTTL must be >= 0")
	}
	if c.ReliabilityConf.RetryBaseDelay < 0 {
		return fmt.Errorf("ReliabilityConf.RetryBaseDelay must be >= 0")
	}
	if c.ReliabilityConf.RetryMaxDelay < 0 {
		return fmt.Errorf("ReliabilityConf.RetryMaxDelay must be >= 0")
	}

	// 校验路由策略枚举
	switch c.RouteConf.UnroutedMessagePolicy {
	case "", "discard", "retry":
		// ok
	default:
		return fmt.Errorf("RouteConf.UnroutedMessagePolicy must be one of [discard, retry], got %q",
			c.RouteConf.UnroutedMessagePolicy)
	}

	return nil
}

// ApplyEnvOverrides 应用环境变量覆盖
// 支持通过环境变量覆盖配置文件中的值
func (c *Config) ApplyEnvOverrides() {
	// Redis 地址
	if addr := os.Getenv("CROSS_CHAIN_REDIS_ADDR"); addr != "" {
		c.SubscribeConf.RedisAddr = addr
	}

	// Redis 密码
	if pwd := os.Getenv("CROSS_CHAIN_REDIS_PASSWORD"); pwd != "" {
		c.SubscribeConf.RedisPassword = pwd
	}

	// 订阅组名
	if group := os.Getenv("CROSS_CHAIN_GROUP_NAME"); group != "" {
		c.SubscribeConf.GroupName = group
	}

	// 可靠性配置
	const boolTrue = "true"
	if v := os.Getenv("CROSS_CHAIN_ENABLE_IDEMPOTENCY"); v != "" {
		c.ReliabilityConf.EnableIdempotency = v == boolTrue || v == "1"
	}
	if v := os.Getenv("CROSS_CHAIN_ENABLE_RETRY"); v != "" {
		c.ReliabilityConf.EnableRetry = v == boolTrue || v == "1"
	}
	if v := os.Getenv("CROSS_CHAIN_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.ReliabilityConf.MaxRetries = n
		}
	}

	// 交易发送配置
	if v := os.Getenv("CROSS_CHAIN_TX_TIMEOUT"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			c.SendTxConf.TxTimeout = n
		}
	}
	if v := os.Getenv("CROSS_CHAIN_WITH_SYNC_RESULT"); v != "" {
		c.SendTxConf.WithSyncResult = v == "true" || v == "1"
	}
}
