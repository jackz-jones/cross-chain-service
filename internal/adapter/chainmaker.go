package adapter

import (
	"fmt"
	"strings"

	"chainmaker.org/chainmaker/common/v2/json"
	"chainmaker.org/chainmaker/pb-go/v2/common"
	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	"github.com/jackz-jones/cross-chain-service/internal/code"
)

// ChainmakerAdapter 长安链适配器
type ChainmakerAdapter struct{}

// NewChainmakerAdapter 创建长安链适配器实例
func NewChainmakerAdapter() *ChainmakerAdapter {
	return &ChainmakerAdapter{}
}

// ChainType 返回链类型标识
func (a *ChainmakerAdapter) ChainType() string {
	return strings.ToLower(chainPb.ChainType_Chainmaker.String())
}

// ParseEvent 解析长安链事件数据
// 流程：JSON → ContractEventInfo → 提取 EventData 列表 → ParsedEvent
func (a *ChainmakerAdapter) ParseEvent(eventData []byte, contractConfs []*chainPb.ContractDesc, contractName string) (*ParsedEvent, error) {
	// 1. 解析长安链事件结构
	var eventInfo common.ContractEventInfo
	if err := json.Unmarshal(eventData, &eventInfo); err != nil {
		return nil, fmt.Errorf("%s chainmaker event info: %v", code.ErrMsgJsonUnmarshal, err)
	}

	// 2. 构造通用事件模型
	// Chainmaker 的事件数据是字符串数组，每个元素代表一个参数
	fields := make(map[string]interface{})
	fields["contractName"] = eventInfo.ContractName
	fields["topic"] = eventInfo.Topic
	fields["txId"] = eventInfo.TxId

	return &ParsedEvent{
		Fields:         fields,
		EventDataItems: eventInfo.EventData,
	}, nil
}
