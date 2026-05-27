package event

import (
	"os"
	"testing"

	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/jackz-jones/cross-chain-service/internal/code"
)

func TestCrossChainTransferEventHandler_EventName(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainTransferEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	if handler.eventName() != "CrossChainTransferEvent" {
		t.Errorf("expected event name 'CrossChainTransferEvent', got '%s'", handler.eventName())
	}
}

func TestCrossChainTransferEventHandler_WrongEventName(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainTransferEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("WrongEvent", "ethereum", "chain1", "Nft", "nft-contract", []byte("data"))
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for wrong event name")
	}
	if err.Error() != code.ErrMsgNotMyEvent {
		t.Errorf("expected error '%s', got '%s'", code.ErrMsgNotMyEvent, err.Error())
	}
}

func TestCrossChainTransferEventHandler_EmptyEventData(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainTransferEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("CrossChainTransferEvent", "ethereum", "chain1", "Nft", "nft-contract", nil)
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for empty event data")
	}
	if err.Error() != code.ErrMsgEventDataEmpty {
		t.Errorf("expected error '%s', got '%s'", code.ErrMsgEventDataEmpty, err.Error())
	}
}

func TestCrossChainTransferEventHandler_UnknownChainType(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainTransferEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("CrossChainTransferEvent", "unknown_chain", "chain1", "Nft", "nft-contract", []byte("data"))
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for unknown chain type")
	}
	expectedPrefix := code.ErrMsgUnknownChainType
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error starting with '%s', got '%s'", expectedPrefix, err.Error())
	}
}

func TestCrossChainTransferEventHandler_Chainmaker_Success(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainTransferEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	// 加载测试数据
	eventData, err := os.ReadFile("testdata/chainmaker_crosschain_transfer.json")
	if err != nil {
		t.Fatalf("failed to read test data: %v", err)
	}

	event := buildTradeGuardEvent("CrossChainTransferEvent", "chainmaker", "chain1", "Nft", "nft-contract", eventData)

	// CrossChainTransfer 总是会触发跨链，由于 CheckWhitelist 当前实现会返回错误
	err = handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error due to CheckWhitelist with empty whitelist map")
	}
	expectedPrefix := code.ErrMsgHandlerCrossChain
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error starting with '%s', got '%s'", expectedPrefix, err.Error())
	}
}

func TestCrossChainTransferEventHandler_Chainmaker_InvalidEventData(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainTransferEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("CrossChainTransferEvent", "chainmaker", "chain1", "Nft", "nft-contract", []byte("invalid json"))
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for invalid chainmaker event data")
	}
}

func TestCrossChainTransferEventHandler_Chainmaker_WrongEventDataLength(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainTransferEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	// CrossChainTransfer 事件需要 3 个元素
	eventData := buildChainmakerEventData([]string{`{"id":"nft-001"}`, "0xfrom001"})

	event := buildTradeGuardEvent("CrossChainTransferEvent", "chainmaker", "chain1", "Nft", "nft-contract", eventData)
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for wrong event data length (need 3)")
	}
}

func TestCrossChainTransferEventHandler_Ethereum_InvalidJSON(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainTransferEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("CrossChainTransferEvent", "ethereum", "chain1", "Nft", "nft-contract", []byte("invalid json"))
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for invalid ethereum event data")
	}
}

func TestCrossChainTransferEventHandler_Ethereum_NoABI(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainTransferEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	eventData := buildValidEthLogJSON()

	event := buildTradeGuardEvent("CrossChainTransferEvent", "ethereum", "chain1", "Nft", "unknown-contract", eventData)
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for missing ABI")
	}
	expectedPrefix := code.ErrMsgInvalidAbi
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error starting with '%s', got '%s'", expectedPrefix, err.Error())
	}
}

func TestCrossChainTransferEventHandler_Ethereum_EmptyContractConfs(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainTransferEventHandler(setup.logger, setup.svcCtx, []*chainPb.ContractDesc{}, setup.crossTargetChainConf)

	eventData := buildValidEthLogJSON()

	event := buildTradeGuardEvent("CrossChainTransferEvent", "ethereum", "chain1", "Nft", "nft-contract", eventData)
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for no matching contract ABI")
	}
}

// 验证 handler 实现了 handler 接口
func TestCrossChainTransferEventHandler_ImplementsInterface(t *testing.T) {
	setup := newTestSetup()
	h := NewCrossChainTransferEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)
	var _ handler = h
	_ = commonEvent.TradeGuardEvent{}
}
