package adapter

import (
	"fmt"
	"strings"

	"chainmaker.org/chainmaker/common/v2/json"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/jackz-jones/cross-chain-service/internal/code"
)

// EthereumAdapter 以太坊链适配器
type EthereumAdapter struct{}

// NewEthereumAdapter 创建以太坊适配器实例
func NewEthereumAdapter() *EthereumAdapter {
	return &EthereumAdapter{}
}

// ChainType 返回链类型标识
func (a *EthereumAdapter) ChainType() string {
	return strings.ToLower(chainPb.ChainType_Ethereum.String())
}

// ParseEvent 解析以太坊事件数据
// 流程：JSON → ethTypes.Log → 查找 ABI → NewEthEventHandler → UnpackIntoMap → Marshal → ParsedEvent
func (a *EthereumAdapter) ParseEvent(eventData []byte, contractConfs []*chainPb.ContractDesc, contractName string) (*ParsedEvent, error) {
	// 1. 解析以太坊事件结构
	var vLog ethTypes.Log
	if err := json.Unmarshal(eventData, &vLog); err != nil {
		return nil, fmt.Errorf("%s ethereum event info: %v", code.ErrMsgJsonUnmarshal, err)
	}

	// 2. 获取合约 ABI
	abi := ""
	for _, contractConf := range contractConfs {
		if contractName == contractConf.ContractName {
			abi = contractConf.Abi
		}
	}

	// 3. 检查 ABI
	if len(abi) == 0 {
		return nil, fmt.Errorf("%s for contract %s", code.ErrMsgInvalidAbi, contractName)
	}

	// 4. 实例化 eth 事件处理器
	ethEventHandler, err := commonEvent.NewEthEventHandler(abi)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", code.ErrMsgNewEthEventHandler, err)
	}

	// 5. 解析 eth 事件到 map[string]interface{}
	result, err := ethEventHandler.UnpackIntoMap(vLog)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", code.ErrMsgUnpackIntoInterface, err)
	}

	// 6. json 序列化成字节（用于后续反序列化到具体结构体）
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", code.ErrMsgJsonMarshal, err)
	}

	return &ParsedEvent{
		Fields:  result,
		RawData: resultBytes,
	}, nil
}
