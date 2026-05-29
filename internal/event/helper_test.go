package event

import (
	"context"
	"errors"
	"testing"
)

func TestSendCrossChainTx_Success(t *testing.T) {
	mockClient := NewMockChainInteractive()

	txId, err := SendCrossChainTx(
		context.Background(),
		TestChainTypeChainmaker, "contract1", "method1", nil,
		0, true, 30, mockClient,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if txId != "mock-tx-id-001" {
		t.Errorf("expected txId 'mock-tx-id-001', got '%s'", txId)
	}

	// 验证调用参数
	if mockClient.GetCallCount() != 1 {
		t.Fatalf("expected 1 call, got %d", mockClient.GetCallCount())
	}
	call := mockClient.GetLastCall()
	if call.ChainName != TestChainTypeChainmaker {
		t.Errorf("expected ChainName '%s', got '%s'", TestChainTypeChainmaker, call.ChainName)
	}
	if call.ContractName != "contract1" {
		t.Errorf("expected ContractName 'contract1', got '%s'", call.ContractName)
	}
	if call.ContractMethod != "method1" {
		t.Errorf("expected ContractMethod 'method1', got '%s'", call.ContractMethod)
	}
}

func TestSendCrossChainTx_GrpcError(t *testing.T) {
	mockClient := NewMockChainInteractive()
	mockClient.CallContractErr = errors.New("connection refused")
	mockClient.CallContractResp = nil

	_, err := SendCrossChainTx(
		context.Background(),
		TestChainTypeChainmaker, "contract1", "method1", nil,
		0, true, 30, mockClient,
	)
	if err == nil {
		t.Fatal("expected error for gRPC failure")
	}
}

func TestSendCrossChainTx_NonSuccessCode(t *testing.T) {
	mockClient := NewMockChainInteractive()
	mockClient.CallContractResp.Code = 500000
	mockClient.CallContractResp.Msg = "internal error"

	_, err := SendCrossChainTx(
		context.Background(),
		TestChainTypeChainmaker, "contract1", "method1", nil,
		0, true, 30, mockClient,
	)
	if err == nil {
		t.Fatal("expected error for non-success response code")
	}
}
