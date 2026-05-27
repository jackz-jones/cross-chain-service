package adapter

import (
	"encoding/json"
	"testing"

	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	"github.com/jackz-jones/cross-chain-service/internal/code"
)

func TestEthereumAdapter_ChainType(t *testing.T) {
	adapter := NewEthereumAdapter()
	if adapter.ChainType() != "ethereum" {
		t.Errorf("expected 'ethereum', got '%s'", adapter.ChainType())
	}
}

func TestEthereumAdapter_ParseEvent_InvalidJSON(t *testing.T) {
	adapter := NewEthereumAdapter()
	_, err := adapter.ParseEvent([]byte("invalid"), nil, "contract1")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestEthereumAdapter_ParseEvent_NoMatchingABI(t *testing.T) {
	adapter := NewEthereumAdapter()

	// 构造有效的以太坊 Log JSON
	ethLog := map[string]interface{}{
		"address":          "0x0000000000000000000000000000000000000000",
		"topics":           []string{},
		"data":             "0x",
		"blockNumber":      "0x1",
		"transactionHash":  "0x0000000000000000000000000000000000000000000000000000000000000000",
		"transactionIndex": "0x0",
		"blockHash":        "0x0000000000000000000000000000000000000000000000000000000000000000",
		"logIndex":         "0x0",
		"removed":          false,
	}
	eventData, _ := json.Marshal(ethLog)

	confs := []*chainPb.ContractDesc{
		{ContractName: "other-contract", Abi: "[]"},
	}

	_, err := adapter.ParseEvent(eventData, confs, "my-contract")
	if err == nil {
		t.Fatal("expected error for no matching ABI")
	}
	if len(err.Error()) < len(code.ErrMsgInvalidAbi) {
		t.Errorf("expected error containing '%s', got '%s'", code.ErrMsgInvalidAbi, err.Error())
	}
}

func TestEthereumAdapter_ParseEvent_EmptyContractConfs(t *testing.T) {
	adapter := NewEthereumAdapter()

	ethLog := map[string]interface{}{
		"address":          "0x0000000000000000000000000000000000000000",
		"topics":           []string{},
		"data":             "0x",
		"blockNumber":      "0x1",
		"transactionHash":  "0x0000000000000000000000000000000000000000000000000000000000000000",
		"transactionIndex": "0x0",
		"blockHash":        "0x0000000000000000000000000000000000000000000000000000000000000000",
		"logIndex":         "0x0",
		"removed":          false,
	}
	eventData, _ := json.Marshal(ethLog)

	_, err := adapter.ParseEvent(eventData, []*chainPb.ContractDesc{}, "contract1")
	if err == nil {
		t.Fatal("expected error for empty contract confs")
	}
}

func TestEthereumAdapter_ImplementsInterface(t *testing.T) {
	var _ ChainAdapter = NewEthereumAdapter()
}

func TestChainmakerAdapter_ChainType(t *testing.T) {
	adapter := NewChainmakerAdapter()
	if adapter.ChainType() != "chainmaker" {
		t.Errorf("expected 'chainmaker', got '%s'", adapter.ChainType())
	}
}

func TestChainmakerAdapter_ParseEvent_Success(t *testing.T) {
	adapter := NewChainmakerAdapter()

	// 构造 Chainmaker ContractEventInfo JSON
	eventInfo := map[string]interface{}{
		"contract_name": "notification-contract",
		"topic":         "EnterpriseNotifiedEvent",
		"tx_id":         "tx-001",
		"event_data":    []string{`{"id":"ent-001","originHash":"0xhash","address":"0xaddr","did":"did:example:123","needCrossChain":true}`, "extra-data"},
	}
	eventData, _ := json.Marshal(eventInfo)

	result, err := adapter.ParseEvent(eventData, nil, "notification-contract")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// 验证 Fields
	if result.Fields["contractName"] != "notification-contract" {
		t.Errorf("expected contractName 'notification-contract', got '%v'", result.Fields["contractName"])
	}
	if result.Fields["topic"] != "EnterpriseNotifiedEvent" {
		t.Errorf("expected topic 'EnterpriseNotifiedEvent', got '%v'", result.Fields["topic"])
	}

	// 验证 EventDataItems
	if len(result.EventDataItems) != 2 {
		t.Fatalf("expected 2 event data items, got %d", len(result.EventDataItems))
	}
}

func TestChainmakerAdapter_ParseEvent_InvalidJSON(t *testing.T) {
	adapter := NewChainmakerAdapter()
	_, err := adapter.ParseEvent([]byte("invalid"), nil, "contract1")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestChainmakerAdapter_ImplementsInterface(t *testing.T) {
	var _ ChainAdapter = NewChainmakerAdapter()
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	eth := NewEthereumAdapter()
	cm := NewChainmakerAdapter()

	reg.Register(eth)
	reg.Register(cm)

	// 大小写不敏感
	adapter, err := reg.Get("Ethereum")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adapter.ChainType() != "ethereum" {
		t.Errorf("expected 'ethereum', got '%s'", adapter.ChainType())
	}

	adapter, err = reg.Get("CHAINMAKER")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adapter.ChainType() != "chainmaker" {
		t.Errorf("expected 'chainmaker', got '%s'", adapter.ChainType())
	}
}

func TestRegistry_GetUnregistered(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Get("unknown")
	if err == nil {
		t.Fatal("expected error for unregistered chain type")
	}
}

func TestGlobalRegistry(t *testing.T) {
	// 注册到全局
	RegisterAdapter(NewEthereumAdapter())
	RegisterAdapter(NewChainmakerAdapter())

	adapter, err := GetAdapter("ethereum")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adapter.ChainType() != "ethereum" {
		t.Errorf("expected 'ethereum', got '%s'", adapter.ChainType())
	}
}

func TestParsedEvent_GetString(t *testing.T) {
	event := &ParsedEvent{
		Fields: map[string]interface{}{
			"name":  "test",
			"count": 42,
		},
	}

	if event.GetString("name") != "test" {
		t.Errorf("expected 'test', got '%s'", event.GetString("name"))
	}
	if event.GetString("count") != "" {
		t.Errorf("expected empty string for non-string field, got '%s'", event.GetString("count"))
	}
	if event.GetString("missing") != "" {
		t.Errorf("expected empty string for missing field, got '%s'", event.GetString("missing"))
	}
}

func TestParsedEvent_GetBool(t *testing.T) {
	event := &ParsedEvent{
		Fields: map[string]interface{}{
			"flag":  true,
			"other": "not-bool",
		},
	}

	if !event.GetBool("flag") {
		t.Error("expected true for 'flag'")
	}
	if event.GetBool("other") {
		t.Error("expected false for non-bool field")
	}
	if event.GetBool("missing") {
		t.Error("expected false for missing field")
	}
}

func TestParsedEvent_GetInt(t *testing.T) {
	event := &ParsedEvent{
		Fields: map[string]interface{}{
			"intVal":   42,
			"floatVal": float64(3.14),
			"int64Val": int64(100),
		},
	}

	if event.GetInt("intVal") != 42 {
		t.Errorf("expected 42, got %d", event.GetInt("intVal"))
	}
	if event.GetInt("floatVal") != 3 {
		t.Errorf("expected 3, got %d", event.GetInt("floatVal"))
	}
	if event.GetInt("int64Val") != 100 {
		t.Errorf("expected 100, got %d", event.GetInt("int64Val"))
	}
	if event.GetInt("missing") != 0 {
		t.Errorf("expected 0, got %d", event.GetInt("missing"))
	}
}

// ==================== Solana 适配器测试 ====================

func TestSolanaAdapter_ChainType(t *testing.T) {
	adapter := NewSolanaAdapter()
	if adapter.ChainType() != "solana" {
		t.Errorf("expected 'solana', got '%s'", adapter.ChainType())
	}
}

func TestSolanaAdapter_ImplementsInterface(t *testing.T) {
	var _ ChainAdapter = NewSolanaAdapter()
}

func TestSolanaAdapter_ParseEvent_Success(t *testing.T) {
	adapter := NewSolanaAdapter()

	// 构造 Solana 事件数据（模拟 blockchain-interactive-service 发布的格式）
	solanaEvent := map[string]interface{}{
		"signature": "5KtPn1LGuxhFiwjxErkxTb3Bqa3MkCYPRKsKAhF3jt2LgZ6yMUXUFhip5GN5z4fVPXJ9QrGFEVLYmUbQYKPHLCz",
		"slot":      float64(123456),
		"logs": []string{
			"Program 11111111111111111111111111111111 invoke [1]",
			`Program log: {"id":"enterprise-001","originHash":"0xabc123","address":"solana_addr_001","did":"did:example:solana","sender":"solana_sender_001","needCrossChain":true}`,
			"Program 11111111111111111111111111111111 success",
		},
		"err":       nil,
		"blockTime": float64(1700000000),
	}
	eventData, _ := json.Marshal(solanaEvent)

	result, err := adapter.ParseEvent(eventData, nil, "notification-contract")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// 验证 Fields
	if result.GetString("id") != "enterprise-001" {
		t.Errorf("expected id 'enterprise-001', got '%s'", result.GetString("id"))
	}
	if result.GetString("originHash") != "0xabc123" {
		t.Errorf("expected originHash '0xabc123', got '%s'", result.GetString("originHash"))
	}
	if result.GetString("address") != "solana_addr_001" {
		t.Errorf("expected address 'solana_addr_001', got '%s'", result.GetString("address"))
	}
	if result.GetString("did") != "did:example:solana" {
		t.Errorf("expected did 'did:example:solana', got '%s'", result.GetString("did"))
	}
	if result.GetString("sender") != "solana_sender_001" {
		t.Errorf("expected sender 'solana_sender_001', got '%s'", result.GetString("sender"))
	}
	if !result.GetBool("needCrossChain") {
		t.Error("expected needCrossChain to be true")
	}

	// 验证 Solana 元数据字段
	if result.GetString("_solana_signature") != "5KtPn1LGuxhFiwjxErkxTb3Bqa3MkCYPRKsKAhF3jt2LgZ6yMUXUFhip5GN5z4fVPXJ9QrGFEVLYmUbQYKPHLCz" {
		t.Errorf("expected _solana_signature, got '%s'", result.GetString("_solana_signature"))
	}

	// 验证 RawData 不为空
	if len(result.RawData) == 0 {
		t.Error("expected non-empty RawData")
	}

	// 验证 EventDataItems 为 nil（Solana 不使用此字段）
	if result.EventDataItems != nil {
		t.Error("expected nil EventDataItems for Solana")
	}
}

func TestSolanaAdapter_ParseEvent_InvalidJSON(t *testing.T) {
	adapter := NewSolanaAdapter()
	_, err := adapter.ParseEvent([]byte("invalid json"), nil, "contract1")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	// 验证错误信息包含 JSON 解析失败的提示
	if len(err.Error()) < len(code.ErrMsgJsonUnmarshal) {
		t.Errorf("expected error containing '%s', got '%s'", code.ErrMsgJsonUnmarshal, err.Error())
	}
}

func TestSolanaAdapter_ParseEvent_NoEventInLogs(t *testing.T) {
	adapter := NewSolanaAdapter()

	// 构造没有事件数据的 Solana 事件
	solanaEvent := map[string]interface{}{
		"signature": "test_sig",
		"slot":      float64(100),
		"logs": []string{
			"Program 11111111111111111111111111111111 invoke [1]",
			"Program 11111111111111111111111111111111 success",
		},
		"err": nil,
	}
	eventData, _ := json.Marshal(solanaEvent)

	_, err := adapter.ParseEvent(eventData, nil, "contract1")
	if err == nil {
		t.Fatal("expected error when no event data in logs")
	}
}

func TestSolanaAdapter_ParseEvent_EmptyLogs(t *testing.T) {
	adapter := NewSolanaAdapter()

	solanaEvent := map[string]interface{}{
		"signature": "test_sig",
		"slot":      float64(100),
		"logs":      []string{},
		"err":       nil,
	}
	eventData, _ := json.Marshal(solanaEvent)

	_, err := adapter.ParseEvent(eventData, nil, "contract1")
	if err == nil {
		t.Fatal("expected error for empty logs")
	}
}

func TestSolanaAdapter_ParseEvent_InvalidJSONInLogs(t *testing.T) {
	adapter := NewSolanaAdapter()

	// 日志中有 "Program log:" 前缀但内容不是有效 JSON
	solanaEvent := map[string]interface{}{
		"signature": "test_sig",
		"slot":      float64(100),
		"logs": []string{
			"Program log: this is not json",
			"Program log: {invalid json content",
		},
		"err": nil,
	}
	eventData, _ := json.Marshal(solanaEvent)

	_, err := adapter.ParseEvent(eventData, nil, "contract1")
	if err == nil {
		t.Fatal("expected error for invalid JSON in logs")
	}
}

func TestSolanaAdapter_ParseEvent_MultipleLogsUsesLast(t *testing.T) {
	adapter := NewSolanaAdapter()

	// 多条有效 JSON 日志，应使用最后一条
	solanaEvent := map[string]interface{}{
		"signature": "test_sig",
		"slot":      float64(100),
		"logs": []string{
			`Program log: {"type":"init","msg":"contract initialized"}`,
			`Program log: {"id":"file-001","originHash":"0xdef456","sender":"solana_sender","msgType":1,"needCrossChain":false}`,
		},
		"err": nil,
	}
	eventData, _ := json.Marshal(solanaEvent)

	result, err := adapter.ParseEvent(eventData, nil, "contract1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 应该使用最后一条有效 JSON
	if result.GetString("id") != "file-001" {
		t.Errorf("expected id 'file-001', got '%s'", result.GetString("id"))
	}
}

func TestSolanaAdapter_GlobalRegistry(t *testing.T) {
	// Solana 适配器应该在 init() 中自动注册到全局注册表

	// 小写查询
	adapter, err := GetAdapter("solana")
	if err != nil {
		t.Fatalf("expected solana adapter registered, got error: %v", err)
	}
	if adapter.ChainType() != "solana" {
		t.Errorf("expected 'solana', got '%s'", adapter.ChainType())
	}

	// 大写查询（大小写不敏感）
	adapter, err = GetAdapter("SOLANA")
	if err != nil {
		t.Fatalf("expected solana adapter for 'SOLANA', got error: %v", err)
	}
	if adapter.ChainType() != "solana" {
		t.Errorf("expected 'solana', got '%s'", adapter.ChainType())
	}

	// 混合大小写查询
	adapter, err = GetAdapter("Solana")
	if err != nil {
		t.Fatalf("expected solana adapter for 'Solana', got error: %v", err)
	}
	if adapter.ChainType() != "solana" {
		t.Errorf("expected 'solana', got '%s'", adapter.ChainType())
	}
}

func TestSolanaAdapter_ParseEvent_NFTTransferEvent(t *testing.T) {
	adapter := NewSolanaAdapter()

	// 构造 NFT 跨链转移事件
	solanaEvent := map[string]interface{}{
		"signature": "nft_transfer_sig",
		"slot":      float64(200000),
		"logs": []string{
			`Program log: {"id":"nft-001","owner":"owner_pubkey","holder":"holder_pubkey","sender":"sender_pubkey","originHash":"0xnfthash","data":"nft_metadata","from":"from_pubkey","to":"to_pubkey","tokenId":"token_001"}`,
		},
		"err": nil,
	}
	eventData, _ := json.Marshal(solanaEvent)

	result, err := adapter.ParseEvent(eventData, nil, "nft-contract")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证所有 NFT 相关字段
	if result.GetString("id") != "nft-001" {
		t.Errorf("expected id 'nft-001', got '%s'", result.GetString("id"))
	}
	if result.GetString("owner") != "owner_pubkey" {
		t.Errorf("expected owner 'owner_pubkey', got '%s'", result.GetString("owner"))
	}
	if result.GetString("holder") != "holder_pubkey" {
		t.Errorf("expected holder 'holder_pubkey', got '%s'", result.GetString("holder"))
	}
	if result.GetString("from") != "from_pubkey" {
		t.Errorf("expected from 'from_pubkey', got '%s'", result.GetString("from"))
	}
	if result.GetString("to") != "to_pubkey" {
		t.Errorf("expected to 'to_pubkey', got '%s'", result.GetString("to"))
	}
	if result.GetString("tokenId") != "token_001" {
		t.Errorf("expected tokenId 'token_001', got '%s'", result.GetString("tokenId"))
	}
}
