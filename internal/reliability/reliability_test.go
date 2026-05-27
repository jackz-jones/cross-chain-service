package reliability

import (
	"context"
	"errors"
	"testing"
	"time"
)

// --- 幂等性测试 ---

func TestInMemoryIdempotencyChecker_NotDuplicate(t *testing.T) {
	checker := NewInMemoryIdempotencyChecker()
	ctx := context.Background()

	dup, err := checker.IsDuplicate(ctx, "event-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dup {
		t.Error("expected not duplicate for new event")
	}
}

func TestInMemoryIdempotencyChecker_MarkAndCheck(t *testing.T) {
	checker := NewInMemoryIdempotencyChecker()
	ctx := context.Background()

	err := checker.MarkProcessed(ctx, "event-001", 1*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dup, err := checker.IsDuplicate(ctx, "event-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dup {
		t.Error("expected duplicate after marking")
	}
}

func TestInMemoryIdempotencyChecker_Expiry(t *testing.T) {
	checker := NewInMemoryIdempotencyChecker()
	ctx := context.Background()

	// 设置极短的 TTL
	err := checker.MarkProcessed(ctx, "event-001", 1*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 等待过期
	time.Sleep(5 * time.Millisecond)

	dup, err := checker.IsDuplicate(ctx, "event-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dup {
		t.Error("expected not duplicate after expiry")
	}
}

// --- 任务持久化测试 ---

func TestInMemoryTaskStore_SaveAndGet(t *testing.T) {
	store := NewInMemoryTaskStore()
	ctx := context.Background()

	task := &CrossChainTask{
		TaskID:       "task-001",
		EventKey:     "0xhash",
		SourceChain:  "chain1",
		TargetChain:  "chain2",
		ContractName: "contract1",
		Method:       "method1",
		State:        TaskStatePending,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := store.Save(ctx, task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := store.Get(ctx, "task-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil task")
	}
	if got.TaskID != "task-001" {
		t.Errorf("expected TaskID 'task-001', got '%s'", got.TaskID)
	}
	if got.State != TaskStatePending {
		t.Errorf("expected state Pending, got %s", got.State)
	}
}

func TestInMemoryTaskStore_UpdateState(t *testing.T) {
	store := NewInMemoryTaskStore()
	ctx := context.Background()

	task := &CrossChainTask{
		TaskID:    "task-001",
		State:     TaskStatePending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = store.Save(ctx, task)

	err := store.UpdateState(ctx, "task-001", TaskStateConfirmed, "tx-123", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := store.Get(ctx, "task-001")
	if got.State != TaskStateConfirmed {
		t.Errorf("expected state Confirmed, got %s", got.State)
	}
	if got.TxID != "tx-123" {
		t.Errorf("expected TxID 'tx-123', got '%s'", got.TxID)
	}
}

func TestInMemoryTaskStore_UpdateState_NotFound(t *testing.T) {
	store := NewInMemoryTaskStore()
	ctx := context.Background()

	err := store.UpdateState(ctx, "nonexistent", TaskStateConfirmed, "", "")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestInMemoryTaskStore_GetPendingTasks(t *testing.T) {
	store := NewInMemoryTaskStore()
	ctx := context.Background()

	_ = store.Save(ctx, &CrossChainTask{TaskID: "task-001", State: TaskStatePending})
	_ = store.Save(ctx, &CrossChainTask{TaskID: "task-002", State: TaskStateSubmitted})
	_ = store.Save(ctx, &CrossChainTask{TaskID: "task-003", State: TaskStateConfirmed})
	_ = store.Save(ctx, &CrossChainTask{TaskID: "task-004", State: TaskStateFailed})

	pending, err := store.GetPendingTasks(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending tasks, got %d", len(pending))
	}
}

func TestInMemoryTaskStore_GetNonExistent(t *testing.T) {
	store := NewInMemoryTaskStore()
	ctx := context.Background()

	got, err := store.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent task")
	}
}

// --- 重试策略测试 ---

func TestRetryStrategy_SuccessOnFirstAttempt(t *testing.T) {
	strategy := NewRetryStrategy(DefaultRetryConfig())

	callCount := 0
	err := strategy.ExecuteWithRetryImmediate(func() error {
		callCount++
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestRetryStrategy_SuccessOnRetry(t *testing.T) {
	strategy := NewRetryStrategy(&RetryConfig{
		MaxRetries: 3,
		BaseDelay:  1 * time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
		Multiplier: 2.0,
	})

	callCount := 0
	err := strategy.ExecuteWithRetryImmediate(func() error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary error")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestRetryStrategy_AllRetriesExhausted(t *testing.T) {
	strategy := NewRetryStrategy(&RetryConfig{
		MaxRetries: 2,
		BaseDelay:  1 * time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
		Multiplier: 2.0,
	})

	callCount := 0
	err := strategy.ExecuteWithRetryImmediate(func() error {
		callCount++
		return errors.New("persistent error")
	})

	if err == nil {
		t.Fatal("expected error after all retries exhausted")
	}
	// 初始尝试 + MaxRetries 次重试 = 3 次
	if callCount != 3 {
		t.Errorf("expected 3 calls (1 initial + 2 retries), got %d", callCount)
	}
}

func TestRetryStrategy_ShouldRetry(t *testing.T) {
	strategy := NewRetryStrategy(&RetryConfig{MaxRetries: 3})

	if !strategy.ShouldRetry(0) {
		t.Error("should retry at count 0")
	}
	if !strategy.ShouldRetry(2) {
		t.Error("should retry at count 2")
	}
	if strategy.ShouldRetry(3) {
		t.Error("should not retry at count 3")
	}
}

func TestRetryStrategy_GetDelay_ExponentialBackoff(t *testing.T) {
	strategy := NewRetryStrategy(&RetryConfig{
		BaseDelay:  1 * time.Second,
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
	})

	d0 := strategy.GetDelay(0)
	d1 := strategy.GetDelay(1)
	d2 := strategy.GetDelay(2)

	if d0 != 1*time.Second {
		t.Errorf("expected 1s delay at retry 0, got %v", d0)
	}
	if d1 != 2*time.Second {
		t.Errorf("expected 2s delay at retry 1, got %v", d1)
	}
	if d2 != 4*time.Second {
		t.Errorf("expected 4s delay at retry 2, got %v", d2)
	}
}

func TestRetryStrategy_GetDelay_MaxCap(t *testing.T) {
	strategy := NewRetryStrategy(&RetryConfig{
		BaseDelay:  1 * time.Second,
		MaxDelay:   5 * time.Second,
		Multiplier: 10.0,
	})

	d := strategy.GetDelay(2) // 1 * 10^2 = 100s, 应被限制为 5s
	if d != 5*time.Second {
		t.Errorf("expected max delay 5s, got %v", d)
	}
}

func TestTaskState_String(t *testing.T) {
	tests := []struct {
		state    TaskState
		expected string
	}{
		{TaskStatePending, "pending"},
		{TaskStateSubmitted, "submitted"},
		{TaskStateConfirmed, "confirmed"},
		{TaskStateFailed, "failed"},
		{TaskState(99), "unknown"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("expected '%s', got '%s'", tt.expected, tt.state.String())
		}
	}
}
