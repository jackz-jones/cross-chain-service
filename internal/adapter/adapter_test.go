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
