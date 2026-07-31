package message

import (
	"testing"

	"github.com/jackz-jones/cross-chain-service/internal/config"
	"github.com/zeromicro/go-zero/core/logx"
)

// newTestRouter 构造一个禁用日志输出的 router，避免测试期打印噪音
func newTestRouter(rules []config.DetailedRouteRule, policy string) *MessageRouter {
	return NewMessageRouter(rules, logx.WithContext(nil), policy)
}

// TestRouter_EventLevel_CaseInsensitive 事件级别路由匹配应大小写不敏感，
// 覆盖 SourceChain / SourceContract / SourceEventName 三个字段。
func TestRouter_EventLevel_CaseInsensitive(t *testing.T) {
	rules := []config.DetailedRouteRule{
		{
			Level:              config.RouteLevelEvent,
			SourceChain:        "ChainA",
			SourceContractType: "NFT",
			SourceEventName:    "Transfer",
			TargetChain:        "ChainB",
			TargetContractType: "NFT",
			TargetMethod:       "OnTransfer",
		},
	}
	r := newTestRouter(rules, "discard")

	msg := NewCrossChainMessage("chaina", "", "nft", "", "transfer", nil)
	targets, err := r.Route(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].TargetChain != "ChainB" || targets[0].Method != "OnTransfer" {
		t.Errorf("unexpected target: %+v", targets[0])
	}
}

// TestRouter_EventLevel_WildcardSourceChain SourceChain 为空视为通配，
// 应匹配任意源链的同名事件。
func TestRouter_EventLevel_WildcardSourceChain(t *testing.T) {
	rules := []config.DetailedRouteRule{
		{
			Level:              config.RouteLevelEvent,
			SourceChain:        "", // 通配
			SourceContractType: "NFT",
			SourceEventName:    "Transfer",
			TargetChain:        "ChainB",
			TargetContractType: "NFT",
			TargetMethod:       "OnTransfer",
		},
	}
	r := newTestRouter(rules, "discard")

	for _, src := range []string{"chainX", "chainY", "ChainZ"} {
		msg := NewCrossChainMessage(src, "", "NFT", "", "Transfer", nil)
		targets, err := r.Route(msg)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", src, err)
		}
		if len(targets) != 1 || targets[0].TargetChain != "ChainB" {
			t.Errorf("source=%s: expected 1 target ChainB, got %+v", src, targets)
		}
	}
}

// TestRouter_MultiTargetBroadcast 一个源事件配置多条同级规则时，
// 应返回全部目标（多目标广播）。
func TestRouter_MultiTargetBroadcast(t *testing.T) {
	rules := []config.DetailedRouteRule{
		{
			Level: config.RouteLevelEvent, SourceChain: "ChainA",
			SourceContractType: "NFT", SourceEventName: "Transfer",
			TargetChain: "ChainB", TargetContractType: "NFT", TargetMethod: "OnTransfer",
		},
		{
			Level: config.RouteLevelEvent, SourceChain: "ChainA",
			SourceContractType: "NFT", SourceEventName: "Transfer",
			TargetChain: "ChainC", TargetContractType: "NFT", TargetMethod: "OnTransfer",
		},
	}
	r := newTestRouter(rules, "discard")

	msg := NewCrossChainMessage("ChainA", "", "NFT", "", "Transfer", nil)
	targets, err := r.Route(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 broadcast targets, got %d: %+v", len(targets), targets)
	}
	got := map[string]bool{targets[0].TargetChain: true, targets[1].TargetChain: true}
	if !got["ChainB"] || !got["ChainC"] {
		t.Errorf("expected broadcast to ChainB & ChainC, got %+v", targets)
	}
}

// TestRouter_PriorityEventOverContract 事件级别路由优先于合约级别路由。
func TestRouter_PriorityEventOverContract(t *testing.T) {
	rules := []config.DetailedRouteRule{
		{
			Level: config.RouteLevelContract, SourceChain: "ChainA",
			SourceContractType: "NFT",
			TargetChain:        "ChainC", TargetContractType: "NFT",
		},
		{
			Level: config.RouteLevelEvent, SourceChain: "ChainA",
			SourceContractType: "NFT", SourceEventName: "Transfer",
			TargetChain: "ChainB", TargetContractType: "NFT", TargetMethod: "OnTransfer",
		},
	}
	r := newTestRouter(rules, "discard")

	msg := NewCrossChainMessage("ChainA", "", "NFT", "", "Transfer", nil)
	targets, err := r.Route(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 || targets[0].TargetChain != "ChainB" {
		t.Errorf("expected event-level match ChainB, got %+v", targets)
	}
}

// TestRouter_UnroutedPolicyRetry 未命中路由 + policy=retry 时应返回错误，
// 用于让上游触发重试或告警；policy=discard 时应返回 (nil, nil)。
func TestRouter_UnroutedPolicyRetry(t *testing.T) {
	r := newTestRouter(nil, "retry")
	msg := NewCrossChainMessage("X", "", "Y", "", "Z", nil)
	targets, err := r.Route(msg)
	if err == nil {
		t.Fatalf("expected error under policy=retry")
	}
	if len(targets) != 0 {
		t.Errorf("expected no targets on unrouted, got %+v", targets)
	}

	r2 := newTestRouter(nil, "discard")
	targets, err = r2.Route(msg)
	if err != nil {
		t.Errorf("expected nil error under policy=discard, got %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected no targets on unrouted, got %+v", targets)
	}
}

// TestRouter_ChainLevelKeepsOriginal 链级别路由应保留消息原有的 TargetContract 与 Method。
func TestRouter_ChainLevelKeepsOriginal(t *testing.T) {
	rules := []config.DetailedRouteRule{
		{
			Level:       config.RouteLevelChain,
			SourceChain: "ChainA",
			TargetChain: "ChainB",
		},
	}
	r := newTestRouter(rules, "discard")

	msg := NewCrossChainMessage("ChainA", "", "NFT", "Bridge", "TransferOut", nil)
	targets, err := r.Route(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].TargetChain != "ChainB" ||
		targets[0].TargetContract != "Bridge" ||
		targets[0].Method != "TransferOut" {
		t.Errorf("chain-level should keep original contract/method, got %+v", targets[0])
	}
}
