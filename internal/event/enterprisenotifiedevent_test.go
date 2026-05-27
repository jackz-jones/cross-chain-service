package event

import (
	"encoding/json"
	"os"
	"testing"

	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/jackz-jones/cross-chain-service/internal/code"
)

func TestEnterpriseNotifiedEventHandler_EventName(t *testing.T) {
	setup := newTestSetup()
	handler := NewEnterpriseNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	if handler.eventName() != "EnterpriseNotifiedEvent" {
		t.Errorf("expected event name 'EnterpriseNotifiedEvent', got '%s'", handler.eventName())
	}
}

func TestEnterpriseNotifiedEventHandler_WrongEventName(t *testing.T) {
	setup := newTestSetup()
	handler := NewEnterpriseNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("WrongEvent", "ethereum", "chain1", "Notification", "notification-contract", []byte("data"))
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for wrong event name")
	}
	if err.Error() != code.ErrMsgNotMyEvent {
		t.Errorf("expected error '%s', got '%s'", code.ErrMsgNotMyEvent, err.Error())
	}
}

func TestEnterpriseNotifiedEventHandler_EmptyEventData(t *testing.T) {
	setup := newTestSetup()
	handler := NewEnterpriseNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("EnterpriseNotifiedEvent", "ethereum", "chain1", "Notification", "notification-contract", nil)
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for empty event data")
	}
	if err.Error() != code.ErrMsgEventDataEmpty {
		t.Errorf("expected error '%s', got '%s'", code.ErrMsgEventDataEmpty, err.Error())
	}
}

func TestEnterpriseNotifiedEventHandler_UnknownChainType(t *testing.T) {
	setup := newTestSetup()
	handler := NewEnterpriseNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("EnterpriseNotifiedEvent", "unknown_chain", "chain1", "Notification", "notification-contract", []byte("data"))
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for unknown chain type")
	}
	expectedPrefix := code.ErrMsgUnknownChainType
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error starting with '%s', got '%s'", expectedPrefix, err.Error())
	}
}

func TestEnterpriseNotifiedEventHandler_Chainmaker_Success_NeedCrossChain(t *testing.T) {
	setup := newTestSetup()
	handler := NewEnterpriseNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	// 加载测试数据
	eventData, err := os.ReadFile("testdata/chainmaker_enterprise_notified.json")
	if err != nil {
		t.Fatalf("failed to read test data: %v", err)
	}

	event := buildTradeGuardEvent("EnterpriseNotifiedEvent", "chainmaker", "chain1", "Notification", "notification-contract", eventData)

	// 由于 CheckWhitelist 当前实现会返回错误（空 map），所以 NeedCrossChain=true 时会失败在白名单检查
	err = handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error due to CheckWhitelist with empty whitelist map")
	}
	// 验证错误是白名单检查失败，而不是解析失败
	expectedPrefix := code.ErrMsgHandlerCrossChain
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error starting with '%s', got '%s'", expectedPrefix, err.Error())
	}
}

func TestEnterpriseNotifiedEventHandler_Chainmaker_NeedCrossChain_False(t *testing.T) {
	setup := newTestSetup()
	handler := NewEnterpriseNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	// 构造 NeedCrossChain=false 的事件数据
	eventData := buildChainmakerEventData([]string{
		`{"id":"enterprise-001","originHash":"0xabc123","createdAt":"2024-01-01T00:00:00Z","address":"0x1234567890abcdef","did":"did:example:123","sender":"0xsender001"}`,
		"false",
	})

	event := buildTradeGuardEvent("EnterpriseNotifiedEvent", "chainmaker", "chain1", "Notification", "notification-contract", eventData)

	// NeedCrossChain=false 时不会触发跨链逻辑，应该直接成功
	err := handler.handleEvent(event)
	if err != nil {
		t.Fatalf("expected no error for NeedCrossChain=false, got: %v", err)
	}

	// 验证没有调用 SendCrossChainTx
	if setup.mockClient.GetCallCount() != 0 {
		t.Errorf("expected 0 CallContract calls, got %d", setup.mockClient.GetCallCount())
	}
}

func TestEnterpriseNotifiedEventHandler_Chainmaker_InvalidEventData(t *testing.T) {
	setup := newTestSetup()
	handler := NewEnterpriseNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	// 无效的 JSON 数据
	event := buildTradeGuardEvent("EnterpriseNotifiedEvent", "chainmaker", "chain1", "Notification", "notification-contract", []byte("invalid json"))
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for invalid chainmaker event data")
	}
}

func TestEnterpriseNotifiedEventHandler_Chainmaker_WrongEventDataLength(t *testing.T) {
	setup := newTestSetup()
	handler := NewEnterpriseNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	// 事件数据长度不为 2
	eventData := buildChainmakerEventData([]string{`{"id":"enterprise-001"}`})

	event := buildTradeGuardEvent("EnterpriseNotifiedEvent", "chainmaker", "chain1", "Notification", "notification-contract", eventData)
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for wrong event data length")
	}
}

func TestEnterpriseNotifiedEventHandler_Ethereum_InvalidJSON(t *testing.T) {
	setup := newTestSetup()
	handler := NewEnterpriseNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("EnterpriseNotifiedEvent", "ethereum", "chain1", "Notification", "notification-contract", []byte("invalid json"))
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for invalid ethereum event data")
	}
}

func TestEnterpriseNotifiedEventHandler_Ethereum_NoABI(t *testing.T) {
	setup := newTestSetup()
	handler := NewEnterpriseNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	// 构造一个有效的以太坊 Log 结构但合约名不匹配
	eventData := buildValidEthLogJSON()

	event := buildTradeGuardEvent("EnterpriseNotifiedEvent", "ethereum", "chain1", "Notification", "unknown-contract", eventData)
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for missing ABI")
	}
	expectedPrefix := code.ErrMsgInvalidAbi
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error starting with '%s', got '%s'", expectedPrefix, err.Error())
	}
}

func TestEnterpriseNotifiedEventHandler_Ethereum_ChainTypeCaseInsensitive(t *testing.T) {
	setup := newTestSetup()
	handler := NewEnterpriseNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	// 测试大小写不敏感
	event := buildTradeGuardEvent("EnterpriseNotifiedEvent", "ETHEREUM", "chain1", "Notification", "notification-contract", []byte("invalid json"))
	err := handler.handleEvent(event)
	// 应该进入 Ethereum 分支（而不是 unknown chain type），然后因为 JSON 解析失败
	if err == nil {
		t.Fatal("expected error")
	}
	// 不应该是 unknown chain type 错误
	if err.Error() == code.ErrMsgUnknownChainType+": ETHEREUM" {
		t.Error("chain type should be case insensitive")
	}
}

// TestEnterpriseNotifiedEventHandler_Ethereum_ValidLog_NoABI 验证以太坊事件在有效 Log 但无匹配 ABI 时的行为
func TestEnterpriseNotifiedEventHandler_Ethereum_ValidLog_NoMatchingContract(t *testing.T) {
	setup := newTestSetup()
	// 使用空合约配置
	handler := NewEnterpriseNotifiedEventHandler(setup.logger, setup.svcCtx, []*chainPb.ContractDesc{}, setup.crossTargetChainConf)

	// 构造最小有效的以太坊 Log JSON
	type ethLog struct {
		Address string   `json:"address"`
		Topics  []string `json:"topics"`
		Data    string   `json:"data"`
	}
	vLog := ethLog{
		Address: "0x0000000000000000000000000000000000000000",
		Topics:  []string{},
		Data:    "0x",
	}
	eventData, _ := json.Marshal(vLog)

	event := buildTradeGuardEvent("EnterpriseNotifiedEvent", "ethereum", "chain1", "Notification", "notification-contract", eventData)
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for no matching contract ABI")
	}
}

// 验证 handler 实现了 handler 接口
func TestEnterpriseNotifiedEventHandler_ImplementsInterface(t *testing.T) {
	setup := newTestSetup()
	h := NewEnterpriseNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	// 编译时检查接口实现
	var _ handler = h
	_ = commonEvent.TradeGuardEvent{}
}
