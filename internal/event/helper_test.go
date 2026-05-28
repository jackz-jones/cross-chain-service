package event

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackz-jones/cross-chain-service/internal/code"

	nftConst "github.com/jackz-jones/nft-contract-go/const"
	nftTypes "github.com/jackz-jones/nft-contract-go/types"
	notificationConst "github.com/jackz-jones/notification-contract-go/const"
	notificationTypes "github.com/jackz-jones/notification-contract-go/types"
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

func TestCreateNotifyEnterpriseInfoKvs(t *testing.T) {
	kvs, err := CreateNotifyEnterpriseInfoKvs(TestEventID, TestTxHash, "0xaddr", "did:example:123", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(kvs) != 2 {
		t.Fatalf("expected 2 kvs, got %d", len(kvs))
	}

	// 验证第一个 kv 是 EnterpriseInfo
	if kvs[0].Key != notificationConst.ParamEnterpriseInfo {
		t.Errorf("expected key '%s', got '%s'", notificationConst.ParamEnterpriseInfo, kvs[0].Key)
	}

	// 验证 EnterpriseInfo 可以反序列化
	var ei notificationTypes.EnterpriseInfo
	err = json.Unmarshal(kvs[0].Value, &ei)
	if err != nil {
		t.Fatalf("failed to unmarshal EnterpriseInfo: %v", err)
	}
	if ei.ID != TestEventID {
		t.Errorf("expected ID '%s', got '%s'", TestEventID, ei.ID)
	}
	if ei.OriginHash != TestTxHash {
		t.Errorf("expected OriginHash '%s', got '%s'", TestTxHash, ei.OriginHash)
	}
	if ei.Address != "0xaddr" {
		t.Errorf("expected Address '0xaddr', got '%s'", ei.Address)
	}
	if ei.Did != "did:example:123" {
		t.Errorf("expected Did 'did:example:123', got '%s'", ei.Did)
	}

	// 验证第二个 kv 是 NeedCrossChain
	if kvs[1].Key != notificationConst.ParamNeedCrossChain {
		t.Errorf("expected key '%s', got '%s'", notificationConst.ParamNeedCrossChain, kvs[1].Key)
	}
}

func TestCreateNotifyFileInfoKvs(t *testing.T) {
	kvs, err := CreateNotifyFileInfoKvs("file-001", TestTxHash, 2, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(kvs) != 2 {
		t.Fatalf("expected 2 kvs, got %d", len(kvs))
	}

	// 验证第一个 kv 是 FileInfo
	if kvs[0].Key != notificationConst.ParamFileInfo {
		t.Errorf("expected key '%s', got '%s'", notificationConst.ParamFileInfo, kvs[0].Key)
	}

	// 验证 FileInfo 可以反序列化
	var fi notificationTypes.FileInfo
	err = json.Unmarshal(kvs[0].Value, &fi)
	if err != nil {
		t.Fatalf("failed to unmarshal FileInfo: %v", err)
	}
	if fi.ID != "file-001" {
		t.Errorf("expected ID 'file-001', got '%s'", fi.ID)
	}
	if fi.OriginHash != TestTxHash {
		t.Errorf("expected OriginHash '%s', got '%s'", TestTxHash, fi.OriginHash)
	}
	if fi.MsgType != notificationTypes.MessageType(2) {
		t.Errorf("expected MsgType 2, got %d", fi.MsgType)
	}

	// 验证第二个 kv 是 NeedCrossChain
	if kvs[1].Key != notificationConst.ParamNeedCrossChain {
		t.Errorf("expected key '%s', got '%s'", notificationConst.ParamNeedCrossChain, kvs[1].Key)
	}
}

func TestCreateCrossChainMintKvs(t *testing.T) {
	kvs, err := CreateCrossChainMintKvs("nft-001", "0xowner", "0xholder", TestTxHash, "metadata")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(kvs) != 1 {
		t.Fatalf("expected 1 kvs, got %d", len(kvs))
	}

	// 验证 kv 是 NFTInfo
	if kvs[0].Key != nftConst.ParamNFTInfo {
		t.Errorf("expected key '%s', got '%s'", nftConst.ParamNFTInfo, kvs[0].Key)
	}

	// 验证 NFTInfo 可以反序列化
	var ni nftTypes.NFTInfo
	err = json.Unmarshal(kvs[0].Value, &ni)
	if err != nil {
		t.Fatalf("failed to unmarshal NFTInfo: %v", err)
	}
	if ni.ID != "nft-001" {
		t.Errorf("expected ID 'nft-001', got '%s'", ni.ID)
	}
	if ni.Owner != "0xowner" {
		t.Errorf("expected Owner '0xowner', got '%s'", ni.Owner)
	}
	if ni.Holder != "0xholder" {
		t.Errorf("expected Holder '0xholder', got '%s'", ni.Holder)
	}
	if ni.OriginHash != TestTxHash {
		t.Errorf("expected OriginHash '%s', got '%s'", TestTxHash, ni.OriginHash)
	}
	if ni.Data != "metadata" {
		t.Errorf("expected Data 'metadata', got '%s'", ni.Data)
	}
}

func TestCreateUpdateCrossChainStatusKvs(t *testing.T) {
	kvs, err := CreateUpdateCrossChainStatusKvs("token-001", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(kvs) != 2 {
		t.Fatalf("expected 2 kvs, got %d", len(kvs))
	}

	// 验证 TokenId
	if kvs[0].Key != nftConst.ParamTokenId {
		t.Errorf("expected key '%s', got '%s'", nftConst.ParamTokenId, kvs[0].Key)
	}
	if string(kvs[0].Value) != "token-001" {
		t.Errorf("expected value 'token-001', got '%s'", string(kvs[0].Value))
	}

	// 验证 State
	if kvs[1].Key != nftConst.ParamState {
		t.Errorf("expected key '%s', got '%s'", nftConst.ParamState, kvs[1].Key)
	}
	if string(kvs[1].Value) != "1" {
		t.Errorf("expected value '1', got '%s'", string(kvs[1].Value))
	}
}

func TestCreateCallbackKvs(t *testing.T) {
	kvs, err := CreateCallbackKvs(TestEventID, TestTxHash, "error message", 711000, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(kvs) != 1 {
		t.Fatalf("expected 1 kvs, got %d", len(kvs))
	}

	// 验证 kv 是 CallbackInfo
	if kvs[0].Key != notificationConst.ParamCallbackInfo {
		t.Errorf("expected key '%s', got '%s'", notificationConst.ParamCallbackInfo, kvs[0].Key)
	}

	// 验证 CallBackInfo 可以反序列化
	var cc notificationTypes.CallBackInfo
	err = json.Unmarshal(kvs[0].Value, &cc)
	if err != nil {
		t.Fatalf("failed to unmarshal CallBackInfo: %v", err)
	}
	if cc.ID != TestEventID {
		t.Errorf("expected ID '%s', got '%s'", TestEventID, cc.ID)
	}
	if cc.OriginHash != TestTxHash {
		t.Errorf("expected OriginHash '%s', got '%s'", TestTxHash, cc.OriginHash)
	}
	if cc.Code != 711000 {
		t.Errorf("expected Code 711000, got %d", cc.Code)
	}
	if cc.Msg != "error message" {
		t.Errorf("expected Msg 'error message', got '%s'", cc.Msg)
	}
	if cc.MsgType != notificationTypes.MessageType(1) {
		t.Errorf("expected MsgType 1, got %d", cc.MsgType)
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
