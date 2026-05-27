package event

import (
	"os"
	"testing"

	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/jackz-jones/cross-chain-service/internal/code"
)

func TestFileNotifiedEventHandler_EventName(t *testing.T) {
	setup := newTestSetup()
	handler := NewFileNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	if handler.eventName() != "FileNotifiedEvent" {
		t.Errorf("expected event name 'FileNotifiedEvent'vent', got '%s'", handler.eventName())
	}
}

func TestFileNotifiedEventHandler_WrongEventName(t *testing.T) {
	setup := newTestSetup()
	handler := NewFileNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("WrongEvent", "ethereum", "chain1", "Notification", "notification-contract", []byte("data"))
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for wrong event name")
	}
	if err.Error() != code.ErrMsgNotMyEvent {
		t.Errorf("expected error '%s', got '%s'", code.ErrMsgNotMyEvent, err.Error())
	}
}

func TestFileNotifiedEventHandler_EmptyEventData(t *testing.T) {
	setup := newTestSetup()
	handler := NewFileNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("FileNotifiedEvent", "ethereum", "chain1", "Notification", "notification-contract", nil)
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for empty event data")
	}
	if err.Error() != code.ErrMsgEventDataEmpty {
		t.Errorf("expected error '%s', got '%s'", code.ErrMsgEventDataEmpty, err.Error())
	}
}

func TestFileNotifiedEventHandler_UnknownChainType(t *testing.T) {
	setup := newTestSetup()
	handler := NewFileNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("FileNotifiedEvent", "unknown_chain", "chain1", "Notification", "notification-contract", []byte("data"))
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for unknown chain type")
	}
	expectedPrefix := code.ErrMsgUnknownChainType
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error starting with '%s', got '%s'", expectedPrefix, err.Error())
	}
}

func TestFileNotifiedEventHandler_Chainmaker_NeedCrossChain_True(t *testing.T) {
	setup := newTestSetup()
	handler := NewFileNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	// 加载测试数据
	eventData, err := os.ReadFile("testdata/chainmaker_file_notified.json")
	if err != nil {
		t.Fatalf("failed to read test data: %v", err)
	}

	event := buildTradeGuardEvent("FileNotifiedEvent", "chainmaker", "chain1", "Notification", "notification-contract", eventData)

	// 由于 CheckWhitelist 当前实现会返回错误（空 map），所以 NeedCrossChain=true 时会失败在白名单检查
	err = handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error due to CheckWhitelist with empty whitelist map")
	}
	expectedPrefix := code.ErrMsgHandlerCrossChain
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error starting with '%s', got '%s'", expectedPrefix, err.Error())
	}
}

func TestFileNotifiedEventHandler_Chainmaker_NeedCrossChain_False(t *testing.T) {
	setup := newTestSetup()
	handler := NewFileNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	// 构造 NeedCrossChain=false 的事件数据
	eventData := buildChainmakerEventData([]string{
		`{"id":"file-001","originHash":"0xdef456","createdAt":"2024-01-01T00:00:00Z","msgType":1,"sender":"0xsender002"}`,
		"false",
	})

	event := buildTradeGuardEvent("FileNotifiedEvent", "chainmaker", "chain1", "Notification", "notification-contract", eventData)

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

func TestFileNotifiedEventHandler_Chainmaker_InvalidEventData(t *testing.T) {
	setup := newTestSetup()
	handler := NewFileNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("FileNotifiedEvent", "chainmaker", "chain1", "Notification", "notification-contract", []byte("invalid json"))
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for invalid chainmaker event data")
	}
}

func TestFileNotifiedEventHandler_Chainmaker_WrongEventDataLength(t *testing.T) {
	setup := newTestSetup()
	handler := NewFileNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	// 事件数据长度不为 2
	eventData := buildChainmakerEventData([]string{`{"id":"file-001"}`})

	event := buildTradeGuardEvent("FileNotifiedEvent", "chainmaker", "chain1", "Notification", "notification-contract", eventData)
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for wrong event data length")
	}
}

func TestFileNotifiedEventHandler_Ethereum_InvalidJSON(t *testing.T) {
	setup := newTestSetup()
	handler := NewFileNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("FileNotifiedEvent", "ethereum", "chain1", "Notification", "notification-contract", []byte("invalid json"))
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for invalid ethereum event data")
	}
}

func TestFileNotifiedEventHandler_Ethereum_NoABI(t *testing.T) {
	setup := newTestSetup()
	handler := NewFileNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	eventData := buildValidEthLogJSON()

	event := buildTradeGuardEvent("FileNotifiedEvent", "ethereum", "chain1", "Notification", "unknown-contract", eventData)
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for missing ABI")
	}
	expectedPrefix := code.ErrMsgInvalidAbi
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error starting with '%s', got '%s'", expectedPrefix, err.Error())
	}
}

func TestFileNotifiedEventHandler_Ethereum_EmptyContractConfs(t *testing.T) {
	setup := newTestSetup()
	handler := NewFileNotifiedEventHandler(setup.logger, setup.svcCtx, []*chainPb.ContractDesc{}, setup.crossTargetChainConf)

	eventData := buildValidEthLogJSON()

	event := buildTradeGuardEvent("FileNotifiedEvent", "ethereum", "chain1", "Notification", "notification-contract", eventData)
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for no matching contract ABI")
	}
}

// 验证 handler 实现了 handler 接口
func TestFileNotifiedEventHandler_ImplementsInterface(t *testing.T) {
	setup := newTestSetup()
	h := NewFileNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)
	var _ handler = h
	_ = commonEvent.TradeGuardEvent{}
}
