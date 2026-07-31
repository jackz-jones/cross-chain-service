// Package reliability 可靠性保障机制
package reliability

import (
	"context"
	"fmt"
	"math"
	"time"
)

// RetryConfig 重试配置
type RetryConfig struct {
	// MaxRetries 最大重试次数
	MaxRetries int
	// BaseDelay 基础延迟时间
	BaseDelay time.Duration
	// MaxDelay 最大延迟时间
	MaxDelay time.Duration
	// Multiplier 退避乘数
	Multiplier float64
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
	}
}

// RetryStrategy 重试策略
type RetryStrategy struct {
	config *RetryConfig
}

// NewRetryStrategy 创建重试策略
func NewRetryStrategy(config *RetryConfig) *RetryStrategy {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &RetryStrategy{config: config}
}

// ShouldRetry 判断是否应该重试
func (s *RetryStrategy) ShouldRetry(retryCount int) bool {
	return retryCount < s.config.MaxRetries
}

// GetDelay 获取当前重试的延迟时间（指数退避）
func (s *RetryStrategy) GetDelay(retryCount int) time.Duration {
	delay := float64(s.config.BaseDelay) * math.Pow(s.config.Multiplier, float64(retryCount))
	if delay > float64(s.config.MaxDelay) {
		delay = float64(s.config.MaxDelay)
	}
	return time.Duration(delay)
}

// GetMaxRetries 获取最大重试次数
func (s *RetryStrategy) GetMaxRetries() int {
	return s.config.MaxRetries
}

// RetryableFunc 可重试的函数类型
type RetryableFunc func() error

// ExecuteWithRetry 执行带重试的操作
// 返回最终错误（如果所有重试都失败）
func (s *RetryStrategy) ExecuteWithRetry(fn RetryableFunc) error {
	var lastErr error

	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// 最后一次尝试失败，不再等待
		if attempt == s.config.MaxRetries {
			break
		}

		// 等待退避时间
		delay := s.GetDelay(attempt)
		time.Sleep(delay)
	}

	return fmt.Errorf("all %d retries exhausted, last error: %v", s.config.MaxRetries+1, lastErr)
}

// ExecuteWithRetryImmediate 执行带重试的操作（不等待，用于测试）
func (s *RetryStrategy) ExecuteWithRetryImmediate(fn RetryableFunc) error {
	var lastErr error

	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
	}

	return fmt.Errorf("all %d retries exhausted, last error: %v", s.config.MaxRetries+1, lastErr)
}

// ExecuteWithRetryCtx 执行带重试的操作（含指数退避 + ctx 中止）
// 相比 ExecuteWithRetry，本方法在退避 sleep 期间会监听 ctx.Done()，
// 使服务退出或上游取消时能够及时中止重试。
func (s *RetryStrategy) ExecuteWithRetryCtx(ctx context.Context, fn RetryableFunc) error {
	var lastErr error

	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		// 每一轮开始前先检查 ctx 是否已取消
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return fmt.Errorf("retry aborted by context: %w (last error: %v)", err, lastErr)
			}
			return fmt.Errorf("retry aborted by context: %w", err)
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// 最后一次尝试失败，不再等待
		if attempt == s.config.MaxRetries {
			break
		}

		// 等待退避时间，同时监听 ctx 取消
		delay := s.GetDelay(attempt)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("retry aborted by context: %w (last error: %v)", ctx.Err(), lastErr)
		case <-timer.C:
		}
	}

	return fmt.Errorf("all %d retries exhausted, last error: %v", s.config.MaxRetries+1, lastErr)
}
