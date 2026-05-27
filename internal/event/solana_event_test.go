package event

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/jackz-jones/cross-chain-service/internal/adapter"
	"github.com/jackz-jones/cross-chain-service/internal/code"
)

func init() {
	// 确保 Solana 适配器已注册（通过导入 adapter 包的 init() 自动完成）
	adapter.RegisterAdapter(adapter.NewSolanaAdapter())
}

// ==================== Solana 企业通知事件测试 ====================

func TestEnterpriseNotifiedEventHandler_Solana_Success_NeedCrossChain(t *testing.T) {
	setup := newTestSetup()
	handler := NewEnterpriseNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	// 加载 Solana 测试数据
	eventData, err := os.ReadFile("testdata/solana_enterprise_notified.json")
	if err != nil {
		t.Fatalf("failed to read test data: %v", err)
	}

	event := buildTradeGuardEvent("EnterpriseNotifiedEvent", "solana", "solana-chain1", "Notification", "notification-contract", eventData)

	// NeedCrossChain=true 时会触发白名单检查，当前实现会返回错误
	err = handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error due to CheckWhitelist with empty whitelist map")
	}
	// 验证错误是跨链处理失败（白名单检查），而不是解析失败
	expectedPrefix := code.ErrMsgHandlerCrossChain
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error starting with '%s', got '%s'", expectedPrefix, err.Error())
	}
}

func TestEnterpriseNotifiedEventHandler_Solana_NeedCrossChain_False(t *testing.T) {
	setup := newTestSetup()
	handler := NewEnterpriseNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	// 构造 NeedCrossChain=false 的 Solana 事件
	eventData := buildSolanaEventData(`{"id":"enterprise-002","originHash":"0xhash002","address":"solana_addr","did":"did:example:002","sender":"solana_sender","needCrossChain":false}`)

	event := buildTradeGuardEvent("EnterpriseNotifiedEvent", "solana", "solana-chain1", "Notification", "notification-contract", eventData)

	// NeedCrossChain=false 时不触发跨链逻辑，应直接成功
	err := handler.handleEvent(event)
	if err != nil {
		t.Fatalf("expected no error for NeedCrossChain=false, got: %v", err)
	}

	// 验证没有调用 SendCrossChainTx
	if setup.mockClient.GetCallCount() != 0 {
		t.Errorf("expected 0 CallContract calls, got %d", setup.mockClient.GetCallCount())
	}
}

func TestEnterpriseNotifiedEventHandler_Solana_InvalidEventData(t *testing.T) {
	setup := newTestSetup()
	handler := NewEnterpriseNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	// 无效的 JSON 数据
	event := buildTradeGuardEvent("EnterpriseNotifiedEvent", "solana", "solana-chain1", "Notification", "notification-contract", []byte("invalid json"))
	err := handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error for invalid solana event data")
	}
}

func TestEnterpriseNotifiedEventHandler_Solana_CaseInsensitive(t *testing.T) {
	setup := newTestSetup()
	handler := NewEnterpriseNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	eventData := buildSolanaEventData(`{"id":"enterprise-003","originHash":"0xhash003","address":"addr","did":"did:003","sender":"sender","needCrossChain":false}`)

	// 测试大小写不敏感 - "SOLANA"
	event := buildTradeGuardEvent("EnterpriseNotifiedEvent", "SOLANA", "solana-chain1", "Notification", "notification-contract", eventData)
	err := handler.handleEvent(event)
	if err != nil {
		t.Fatalf("expected no error for SOLANA (case insensitive), got: %v", err)
	}

	// 测试大小写不敏感 - "Solana"
	event = buildTradeGuardEvent("EnterpriseNotifiedEvent", "Solana", "solana-chain1", "Notification", "notification-contract", eventData)
	err = handler.handleEvent(event)
	if err != nil {
		t.Fatalf("expected no error for Solana (case insensitive), got: %v", err)
	}
}

// ==================== Solana 文件通知事件测试 ====================

func TestFileNotifiedEventHandler_Solana_Success_NeedCrossChain(t *testing.T) {
	setup := newTestSetup()
	handler := NewFileNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	eventData, err := os.ReadFile("testdata/solana_file_notified.json")
	if err != nil {
		t.Fatalf("failed to read test data: %v", err)
	}

	event := buildTradeGuardEvent("FileNotifiedEvent", "solana", "solana-chain1", "Notification", "notification-contract", eventData)

	// NeedCrossChain=true 时会触发白名单检查
	err = handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error due to CheckWhitelist")
	}
	expectedPrefix := code.ErrMsgHandlerCrossChain
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error starting with '%s', got '%s'", expectedPrefix, err.Error())
	}
}

func TestFileNotifiedEventHandler_Solana_NeedCrossChain_False(t *testing.T) {
	setup := newTestSetup()
	handler := NewFileNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	eventData := buildSolanaEventData(`{"id":"file-002","originHash":"0xfilehash","sender":"solana_sender","msgType":2,"needCrossChain":false}`)

	event := buildTradeGuardEvent("FileNotifiedEvent", "solana", "solana-chain1", "Notification", "notification-contract", eventData)

	err := handler.handleEvent(event)
	if err != nil {
		t.Fatalf("expected no error for NeedCrossChain=false, got: %v", err)
	}
}

// ==================== Solana 跨链转移事件测试 ====================

func TestCrossChainTransferEventHandler_Solana_Success(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainTransferEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	eventData, err := os.ReadFile("testdata/solana_crosschain_transfer.json")
	if err != nil {
		t.Fatalf("failed to read test data: %v", err)
	}

	event := buildTradeGuardEvent("CrossChainTransferEvent", "solana", "solana-chain1", "Nft", "nft-contract", eventData)

	// 会触发白名单检查
	err = handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error due to CheckWhitelist")
	}
	expectedPrefix := code.ErrMsgHandlerCrossChain
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error starting with '%s', got '%s'", expectedPrefix, err.Error())
	}
}

// ==================== Solana 跨链 Mint 事件测试 ====================

func TestCrossChainMintEventHandler_Solana_Success(t *testing.T) {
	setup := newTestSetup()
	handler := NewCrossChainMintEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf)

	eventData, err := os.ReadFile("testdata/solana_crosschain_mint.json")
	if err != nil {
		t.Fatalf("failed to read test data: %v", err)
	}

	event := buildTradeGuardEvent("CrossChainMintEvent", "solana", "solana-chain1", "Nft", "nft-contract", eventData)

	// 会触发白名单检查
	err = handler.handleEvent(event)
	if err == nil {
		t.Fatal("expected error due to CheckWhitelist")
	}
	expectedPrefix := code.ErrMsgHandlerCrossChain
	if len(err.Error()) < len(expectedPrefix) || err.Error()[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected error starting with '%s', got '%s'", expectedPrefix, err.Error())
	}
}

// ==================== Solana 适配器在事件处理器中的集成验证 ====================

func TestSolanaAdapter_IntegrationWithDispatcher(t *testing.T) {
	setup := newTestSetup()

	// 创建所有事件处理器
	handlers := []handler{
		NewEnterpriseNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf),
		NewFileNotifiedEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf),
		NewCrossChainTransferEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf),
		NewCrossChainMintEventHandler(setup.logger, setup.svcCtx, setup.contractConfs, setup.crossTargetChainConf),
	}

	dispatcher := newHandlerDispatcher(handlers, setup.logger)

	// 验证 Solana 事件能被正确分发到对应处理器
	testCases := []struct {
		name      string
		eventName string
		eventData string
	}{
		{
			name:      "EnterpriseNotified",
			eventName: "EnterpriseNotifiedEvent",
			eventData: `{"id":"ent-sol","originHash":"0x111","address":"addr","did":"did:sol","sender":"sender","needCrossChain":false}`,
		},
		{
			name:      "FileNotified",
			eventName: "FileNotifiedEvent",
			eventData: `{"id":"file-sol","originHash":"0x222","sender":"sender","msgType":1,"needCrossChain":false}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			solanaEventData := buildSolanaEventData(tc.eventData)
			event := buildTradeGuardEvent(tc.eventName, "solana", "solana-chain1", "Notification", "notification-contract", solanaEventData)

			err := dispatcher.dispatchTopicHandler(event)
			if err != nil {
				t.Fatalf("expected no error for %s with NeedCrossChain=false, got: %v", tc.name, err)
			}
		})
	}
}

// ==================== 辅助函数 ====================

// buildSolanaEventData 构造 Solana 事件数据（模拟 blockchain-interactive-service 发布的格式）
func buildSolanaEventData(eventJSON string) []byte {
	type solanaEvent struct {
		Signature string   `json:"signature"`
		Slot      int      `json:"slot"`
		Logs      []string `json:"logs"`
		Err       *string  `json:"err"`
	}

	ev := solanaEvent{
		Signature: "test_sig",
		Slot:      100,
		Logs:      []string{"Program log: " + eventJSON},
		Err:       nil,
	}

	data, _ := json.Marshal(ev)
	return data
}
