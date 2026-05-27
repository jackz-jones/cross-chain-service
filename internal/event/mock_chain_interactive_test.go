package event

import (
	"context"
	"sync"

	"github.com/jackz-jones/blockchain-interactive-service/chaininteractive"
	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	"google.golang.org/grpc"
)

// CallContractCall 记录一次 CallContract 调用的参数
type CallContractCall struct {
	ChainName      string
	ContractName   string
	ContractMethod string
	KvPairs        []*chainPb.KeyValuePair
	MethodType     chainPb.MethodType
	WithSyncResult bool
	TxTimeout      int64
}

// MockChainInteractive 模拟 ChainInteractive 接口，用于测试
type MockChainInteractive struct {
	mu sync.Mutex

	// CallContractCalls 记录所有 CallContract 调用
	CallContractCalls []CallContractCall

	// CallContractResp 配置 CallContract 返回值
	CallContractResp *chainPb.TxResponse

	// CallContractErr 配置 CallContract 返回错误
	CallContractErr error

	// CallContractRespFunc 动态返回值函数（优先级高于 CallContractResp）
	CallContractRespFunc func(req *chaininteractive.CallContractRequest) (*chainPb.TxResponse, error)

	// GetAvailableChainAndContractNamesResp 配置返回值
	GetAvailableChainAndContractNamesResp *chaininteractive.GetAvailableChainAndContractNamesResponse

	// GetAvailableChainAndContractNamesErr 配置返回错误
	GetAvailableChainAndContractNamesErr error
}

// NewMockChainInteractive 创建默认成功的 Mock 客户端
func NewMockChainInteractive() *MockChainInteractive {
	return &MockChainInteractive{
		CallContractResp: &chainPb.TxResponse{
			Code: int32(200000),
			Msg:  "success",
			Data: &chainPb.TxData{
				TxId: "mock-tx-id-001",
			},
		},
	}
}

// CallContract 模拟合约调用
func (m *MockChainInteractive) CallContract(ctx context.Context, in *chaininteractive.CallContractRequest,
	opts ...grpc.CallOption) (*chainPb.TxResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 记录调用参数
	call := CallContractCall{
		ChainName:      in.ChainName,
		ContractName:   in.ContractName,
		ContractMethod: in.ContractMethod,
		KvPairs:        in.KvPairs,
		MethodType:     in.MethodType,
		WithSyncResult: in.WithSyncResult,
		TxTimeout:      in.TxTimeout,
	}
	m.CallContractCalls = append(m.CallContractCalls, call)

	// 如果有动态返回函数，优先使用
	if m.CallContractRespFunc != nil {
		return m.CallContractRespFunc(in)
	}

	return m.CallContractResp, m.CallContractErr
}

// GetTxByTxId 模拟查询交易
func (m *MockChainInteractive) GetTxByTxId(ctx context.Context,
	in *chaininteractive.GetTxByTxIdRequest, opts ...grpc.CallOption) (*chainPb.TxResponse, error) {
	return &chainPb.TxResponse{
		Code: int32(200000),
		Msg:  "success",
	}, nil
}

// GetAvailableChainAndContractNames 模拟获取链配置
func (m *MockChainInteractive) GetAvailableChainAndContractNames(ctx context.Context,
	in *chaininteractive.GetAvailableChainAndContractNamesRequest,
	opts ...grpc.CallOption) (*chaininteractive.GetAvailableChainAndContractNamesResponse, error) {
	if m.GetAvailableChainAndContractNamesErr != nil {
		return nil, m.GetAvailableChainAndContractNamesErr
	}
	if m.GetAvailableChainAndContractNamesResp != nil {
		return m.GetAvailableChainAndContractNamesResp, nil
	}
	return &chaininteractive.GetAvailableChainAndContractNamesResponse{
		Code: int32(200000),
		Msg:  "success",
	}, nil
}

// Reset 重置所有记录
func (m *MockChainInteractive) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallContractCalls = nil
}

// GetCallCount 获取 CallContract 调用次数
func (m *MockChainInteractive) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.CallContractCalls)
}

// GetLastCall 获取最后一次 CallContract 调用
func (m *MockChainInteractive) GetLastCall() *CallContractCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.CallContractCalls) == 0 {
		return nil
	}
	return &m.CallContractCalls[len(m.CallContractCalls)-1]
}
