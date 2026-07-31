package reliability

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestRetryStrategy 构造一个测试用的、退避时间较短的策略
func newTestRetryStrategy(maxRetries int) *RetryStrategy {
	return NewRetryStrategy(&RetryConfig{
		MaxRetries: maxRetries,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   50 * time.Millisecond,
		Multiplier: 2.0,
	})
}

// TestExecuteWithRetryCtx_FirstAttemptSuccess 首次调用即返回 nil error，
// 不应触发重试。
func TestExecuteWithRetryCtx_FirstAttemptSuccess(t *testing.T) {
	strategy := newTestRetryStrategy(3)
	var calls int32

	err := strategy.ExecuteWithRetryCtx(context.Background(), func() error {
		atomic.AddInt32(&calls, 1)
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected exactly 1 call, got %d", got)
	}
}

// TestExecuteWithRetryCtx_RetryUntilSuccess 前两次失败、第三次成功，
// 应发生重试且最终返回 nil。
func TestExecuteWithRetryCtx_RetryUntilSuccess(t *testing.T) {
	strategy := newTestRetryStrategy(3)
	var calls int32

	err := strategy.ExecuteWithRetryCtx(context.Background(), func() error {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("expected 3 calls, got %d", got)
	}
}

// TestExecuteWithRetryCtx_AllAttemptsFail 全部尝试都失败时，
// 应返回聚合错误并耗尽 MaxRetries+1 次。
func TestExecuteWithRetryCtx_AllAttemptsFail(t *testing.T) {
	strategy := newTestRetryStrategy(2)
	var calls int32

	err := strategy.ExecuteWithRetryCtx(context.Background(), func() error {
		atomic.AddInt32(&calls, 1)
		return errors.New("boom")
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if got := atomic.LoadInt32(&calls); got != 3 { // MaxRetries=2 => 3 attempts
		t.Errorf("expected 3 calls, got %d", got)
	}
}

// TestExecuteWithRetryCtx_CancelDuringBackoff 在退避 sleep 期间取消 ctx，
// 应立即中止并返回 context.Canceled 包装错误。
func TestExecuteWithRetryCtx_CancelDuringBackoff(t *testing.T) {
	strategy := NewRetryStrategy(&RetryConfig{
		MaxRetries: 5,
		BaseDelay:  200 * time.Millisecond,
		MaxDelay:   1 * time.Second,
		Multiplier: 2.0,
	})

	ctx, cancel := context.WithCancel(context.Background())
	// 让第一次失败后进入 200ms 退避，此时取消
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	var calls int32
	start := time.Now()
	err := strategy.ExecuteWithRetryCtx(ctx, func() error {
		atomic.AddInt32(&calls, 1)
		return errors.New("always fail")
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected error due to ctx cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		// ExecuteWithRetryCtx 使用 %w 包装 ctx.Err()，errors.Is 应能识别
		t.Errorf("expected wrapped context.Canceled, got: %v", err)
	}
	if !strings.Contains(err.Error(), "retry aborted by context") {
		t.Errorf("expected 'retry aborted by context' prefix, got: %v", err)
	}
	// 至少调用过一次，但不应等到退避完全结束
	if got := atomic.LoadInt32(&calls); got < 1 {
		t.Errorf("expected at least 1 call, got %d", got)
	}
	if elapsed >= 200*time.Millisecond {
		t.Errorf("expected cancel to abort before 200ms backoff, elapsed=%v", elapsed)
	}
}

// TestExecuteWithRetryCtx_PreCancelled 传入已取消的 ctx 时，
// 应在第一轮循环开始时立即中止。
func TestExecuteWithRetryCtx_PreCancelled(t *testing.T) {
	strategy := newTestRetryStrategy(3)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var calls int32
	err := strategy.ExecuteWithRetryCtx(ctx, func() error {
		atomic.AddInt32(&calls, 1)
		return errors.New("should not be called")
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected wrapped context.Canceled, got: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Errorf("expected 0 calls with pre-cancelled ctx, got %d", got)
	}
}
