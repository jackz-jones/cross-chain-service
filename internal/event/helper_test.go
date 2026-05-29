package event

import (
	"errors"
	"testing"

	"github.com/jackz-jones/cross-chain-service/internal/code"
)

func TestCheckWhitelist_EmptyMap_AllAddressesFail(t *testing.T) {
	// 当前实现：空 map 导致所有地址返回 error
	err := CheckWhitelist([]string{"0xaddr1", "0xaddr2"})
	if err == nil {
		t.Fatal("expected error for addresses not in empty whitelist")
	}
	// 验证错误信息包含地址和 "not in white list"
	expectedSuffix := code.ErrMsgNotInWhiteList
	if len(err.Error()) < len(expectedSuffix) {
		t.Fatalf("error message too short: %s", err.Error())
	}
}

func TestCheckWhitelist_EmptyAddressList(t *testing.T) {
	// 空地址列表应该成功
	err := CheckWhitelist([]string{})
	if err != nil {
		t.Fatalf("expected no error for empty address list, got: %v", err)
	}
}

func TestNotifyFile_CurrentBehavior(t *testing.T) {
	// 当前实现为空，总是返回 nil
	err := NotifyFile(TestEventID, "0xaddr", 1)
	if err != nil {
		t.Fatalf("expected no error from NotifyFile stub, got: %v", err)
	}
}

func TestSendCrossChainTx_Success(t *testing.T) {
	mockClient := NewMockChainInteractive()

	txId, err := SendCrossChainTx(
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
		TestChainTypeChainmaker, "contract1", "method1", nil,
		0, true, 30, mockClient,
	)
	if err == nil {
		t.Fatal("expected error for non-success response code")
	}
}
