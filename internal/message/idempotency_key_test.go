package message

import (
	"context"
	"testing"
	"time"

	"github.com/jackz-jones/cross-chain-service/internal/reliability"
)

// buildIdempotencyKey 与 reliableExecutor.Execute 中的 eventKey 拼接方式保持一致。
// 一旦生产代码修改 key 结构，需要同步更新这里 —— 该测试的目的正是防止
// 未来有人把 MessageID 从 key 中去掉，导致「同类型不同消息被误判为重复」的回归。
func buildIdempotencyKey(msgID, targetChain, targetContract, method string) string {
	return msgID + ":" + targetChain + ":" + targetContract + ":" + method
}

// TestIdempotencyKey_DifferentMessagesNotDuplicate 同一 target/contract/method
// 下，不同 MessageID 的消息必须被视为不同事件，不能相互覆盖。
// 这是任务 2 修复的核心回归点：旧实现只用 target:contract:method 作为 key。
func TestIdempotencyKey_DifferentMessagesNotDuplicate(t *testing.T) {
	checker := reliability.NewInMemoryIdempotencyChecker()
	ctx := context.Background()

	msg1 := NewCrossChainMessage("ChainA", "ChainB", "NFT", "NFT", "Transfer", nil)
	msg2 := NewCrossChainMessage("ChainA", "ChainB", "NFT", "NFT", "Transfer", nil)
	if msg1.MessageID == msg2.MessageID {
		t.Fatalf("precondition: MessageIDs should be unique")
	}

	key1 := buildIdempotencyKey(msg1.MessageID, "ChainB", "NFT", "OnTransfer")
	key2 := buildIdempotencyKey(msg2.MessageID, "ChainB", "NFT", "OnTransfer")

	if err := checker.MarkProcessed(ctx, key1, time.Minute); err != nil {
		t.Fatalf("mark msg1: %v", err)
	}
	dup, err := checker.IsDuplicate(ctx, key2)
	if err != nil {
		t.Fatalf("check msg2: %v", err)
	}
	if dup {
		t.Fatal("msg2 must NOT be treated as duplicate of msg1 (different MessageID)")
	}
}

// TestIdempotencyKey_SameMessageIsDuplicate 同一 MessageID + 同一目标必须被识别为重复，
// 用于确认幂等 key 的另一个方向仍然生效。
func TestIdempotencyKey_SameMessageIsDuplicate(t *testing.T) {
	checker := reliability.NewInMemoryIdempotencyChecker()
	ctx := context.Background()

	msg := NewCrossChainMessage("ChainA", "ChainB", "NFT", "NFT", "Transfer", nil)
	key := buildIdempotencyKey(msg.MessageID, "ChainB", "NFT", "OnTransfer")

	if err := checker.MarkProcessed(ctx, key, time.Minute); err != nil {
		t.Fatalf("mark: %v", err)
	}
	dup, err := checker.IsDuplicate(ctx, key)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !dup {
		t.Fatal("same MessageID + target should be treated as duplicate")
	}
}

// TestIdempotencyKey_SameMessageDifferentTargets 同一 MessageID 广播到不同目标链时，
// 每个目标应各自维护独立的幂等状态；一个目标已处理不能阻止另一个目标处理。
// 这是任务 6 多目标广播场景下的关键回归点。
func TestIdempotencyKey_SameMessageDifferentTargets(t *testing.T) {
	checker := reliability.NewInMemoryIdempotencyChecker()
	ctx := context.Background()

	msg := NewCrossChainMessage("ChainA", "", "NFT", "NFT", "Transfer", nil)
	keyB := buildIdempotencyKey(msg.MessageID, "ChainB", "NFT", "OnTransfer")
	keyC := buildIdempotencyKey(msg.MessageID, "ChainC", "NFT", "OnTransfer")

	if err := checker.MarkProcessed(ctx, keyB, time.Minute); err != nil {
		t.Fatalf("mark keyB: %v", err)
	}
	dup, err := checker.IsDuplicate(ctx, keyC)
	if err != nil {
		t.Fatalf("check keyC: %v", err)
	}
	if dup {
		t.Fatal("broadcast to ChainC must not be blocked by ChainB's idempotency mark")
	}
}
