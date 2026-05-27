package event

import (
	"os"
	"testing"

	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/jackz-jones/cross-chain-service/internal/code"
)

func TestCrossChainMintEventHandler_EventName(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainMintEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	if handler.eventName() != "CrossChainMintEvent" {
		t.Errorf("expected event name 'CrossChainMintEvent', got '%s'", handler.eventName())
	}
}

func TestCrossChainMintEventHandler_WrongEventName(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainMintEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("WrongEvent", "ethereum", "chain1", "Nft", "nft-contract", []byte("data"))
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for wrong event name")
	}
	if err.Error() != code.ErrMsgNotMyEvent {
		t.Errorf("expected error '%s', got '%s'", code.ErrMsgNotMyEvent, err.Error())
	}
}

func TestCrossChainMintEventHandler_EmptyEventData(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainMintEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("CrossChainMintEvent", "ethereum", "chain1", "Nft", "nft-contract", nil)
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for empty event data")
	}
	if err.Error() != code.ErrMsgEventDataEmpty {
		t.Errorf("expected error '%s', got '%s'", code.ErrMsgEventDataEmpty, err.Error())
	}
}

func TestCrossChainMintEventHandler_UnknownChainType(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainMintEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("CrossChainMintEvent", "unknown_chain", "chain1", "Nft", "nft-contract", []byte("data"))
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for unknown chain type")
	}
	expectedPrefix := code.ErrMsgUnknownChainType
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error starting with '%s', got '%s'", expectedPrefix, err.Error())
	}
}

func TestCrossChainMintEventHandler_Chainmaker_Success(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainMintEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	// 加载测试数据
	eventData, err := os.ReadFile("testdata/chainmaker_crosschain_mint.json")
	if err != nil {
		t.Fatalf("failed to read test data: %v", err)
	}

	event := buildTradeGuardEvent("CrossChainMintEvent", "chainmaker", "chain1", "Nft", "nft-contract", eventData)

	// CrossChainMint 总是会触发跨链，由于 CheckWhitelist 当前实现会返回错误
	err = handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error due to CheckWhitelist with empty whitelist map")
	}
	expectedPrefix := code.ErrMsgHandlerCrossChain
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error starting with '%s', got '%s'", expectedPrefix, err.Error())
	}
}

func TestCrossChainMintEventHandler_Chainmaker_InvalidEventData(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainMintEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("CrossChainMintEvent", "chainmaker", "chain1", "Nft", "nft-contract", []byte("invalid json"))
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for invalid chainmaker event data")
	}
}

func TestCrossChainMintEventHandler_Chainmaker_WrongEventDataLength(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainMintEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	// CrossChainMint 事件需要 1 个元素
	eventData := buildChainmakerEventData([]string{`{"id":"nft-001"}`, "extra-data"})

	event := buildTradeGuardEvent("CrossChainMintEvent", "chainmaker", "chain1", "Nft", "nft-contract", eventData)
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for wrong event data length (need 1)")
	}
}

func TestCrossChainMintEventHandler_Ethereum_InvalidJSON(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainMintEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	event := buildTradeGuardEvent("CrossChainMintEvent", "ethereum", "chain1", "Nft", "nft-contract", []byte("invalid json"))
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for invalid ethereum event data")
	}
}

func TestCrossChainMintEventHandler_Ethereum_NoABI(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainMintEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	eventData := buildValidEthLogJSON()

	event := buildTradeGuardEvent("CrossChainMintEvent", "ethereum", "chain1", "Nft", "unknown-contract", eventData)
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for missing ABI")
	}
	expectedPrefix := code.ErrMsgInvalidAbi
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error starting with '%s', got '%s'", expectedPrefix, err.Error())
	}
}

func TestCrossChainMintEventHandler_Ethereum_EmptyContractConfs(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainMintEventHandler(setup.logger, setup.svcCtx, []*chainPb.ContractDesc{}, setup.crossTargetChainConf)

	eventData := buildValidEthLogJSON()

	event := buildTradeGuardEvent("CrossChainMintEvent", "ethereum", "chain1", "Nft", "nft-contract", eventData)
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for no matching contract ABI")
	}
}

// 验证 handler 实现了 handler 接口
func TestCrossChainMintEventHandler_ImplementsInterface(t *testing.T) {
	setup := newTestSetup()
	h := NewCrossChainMintEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)
	var _ handler = h
	_ = commonEvent.TradeGuardEvent{}
}
