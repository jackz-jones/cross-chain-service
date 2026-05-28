package config

import (
	"os"
	"testing"
)

const (
	testRedisAddr    = "localhost:6379"
	testOriginalAddr = "original:6379"
	testEnvRedisAddr = "env-redis:6379"
	testGrpcEndpoint = "localhost:8080"
)

func TestConfig_Validate_Valid(t *testing.T) {
	c := &Config{}
	c.SubscribeConf.RedisAddr = testRedisAddr
	c.ExternalGrpcConfs = map[string]*ExternalGrpcConf{
		"chain": {Endpoint: testGrpcEndpoint},
	}

	err := c.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfig_Validate_MissingRedisAddr(t *testing.T) {
	c := &Config{}
	c.ExternalGrpcConfs = map[string]*ExternalGrpcConf{
		"chain": {Endpoint: "localhost:8080"},
	}

	err := c.Validate()
	if err == nil {
		t.Fatal("expected error for missing SubscribeConf.RedisAddr")
	}
}

func TestConfig_Validate_EmptyExternalGrpc(t *testing.T) {
	c := &Config{}
	c.SubscribeConf.RedisAddr = testRedisAddr
	c.ExternalGrpcConfs = map[string]*ExternalGrpcConf{}

	err := c.Validate()
	if err == nil {
		t.Fatal("expected error for empty ExternalGrpcConfs")
	}
}

func TestConfig_Validate_MissingEndpoint(t *testing.T) {
	c := &Config{}
	c.SubscribeConf.RedisAddr = testRedisAddr
	c.ExternalGrpcConfs = map[string]*ExternalGrpcConf{
		"chain": {Endpoint: ""},
	}

	err := c.Validate()
	if err == nil {
		t.Fatal("expected error for missing Endpoint")
	}
}

func TestConfig_Validate_InvalidRetryConfig(t *testing.T) {
	c := &Config{}
	c.SubscribeConf.RedisAddr = testRedisAddr
	c.ExternalGrpcConfs = map[string]*ExternalGrpcConf{
		"chain": {Endpoint: testGrpcEndpoint},
	}
	c.ReliabilityConf.MaxRetries = -1

	err := c.Validate()
	if err == nil {
		t.Fatal("expected error for negative MaxRetries")
	}
}

func TestConfig_Validate_InvalidRouteConf(t *testing.T) {
	c := &Config{}
	c.SubscribeConf.RedisAddr = testRedisAddr
	c.ExternalGrpcConfs = map[string]*ExternalGrpcConf{
		"chain": {Endpoint: testGrpcEndpoint},
	}
	c.RouteConf = []RouteRule{
		{SourceChain: "", TargetChains: []string{"chain2"}},
	}

	err := c.Validate()
	if err == nil {
		t.Fatal("expected error for empty SourceChain")
	}
}

func TestConfig_ApplyEnvOverrides(t *testing.T) {
	c := &Config{}

	os.Setenv("CROSS_CHAIN_REDIS_ADDR", testEnvRedisAddr)
	os.Setenv("CROSS_CHAIN_ENABLE_RETRY", "true")
	os.Setenv("CROSS_CHAIN_MAX_RETRIES", "5")
	os.Setenv("CROSS_CHAIN_TX_TIMEOUT", "30")
	defer func() {
		os.Unsetenv("CROSS_CHAIN_REDIS_ADDR")
		os.Unsetenv("CROSS_CHAIN_ENABLE_RETRY")
		os.Unsetenv("CROSS_CHAIN_MAX_RETRIES")
		os.Unsetenv("CROSS_CHAIN_TX_TIMEOUT")
	}()

	c.ApplyEnvOverrides()

	if c.SubscribeConf.RedisAddr != testEnvRedisAddr {
		t.Errorf("expected RedisAddr '%s', got '%s'", testEnvRedisAddr, c.SubscribeConf.RedisAddr)
	}
	if !c.ReliabilityConf.EnableRetry {
		t.Error("expected EnableRetry to be true")
	}
	if c.ReliabilityConf.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", c.ReliabilityConf.MaxRetries)
	}
	if c.SendTxConf.TxTimeout != 30 {
		t.Errorf("expected TxTimeout 30, got %d", c.SendTxConf.TxTimeout)
	}
}

func TestConfig_ApplyEnvOverrides_NoEnv(t *testing.T) {
	c := &Config{}
	c.SubscribeConf.RedisAddr = testOriginalAddr

	// 确保环境变量不存在
	os.Unsetenv("CROSS_CHAIN_REDIS_ADDR")

	c.ApplyEnvOverrides()

	if c.SubscribeConf.RedisAddr != testOriginalAddr {
		t.Errorf("expected original addr, got '%s'", c.SubscribeConf.RedisAddr)
	}
}
