package adapter

import (
	"encoding/json"
	"fmt"
	"strings"

	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	"github.com/jackz-jones/cross-chain-service/internal/code"
)

// Solana 事件日志前缀常量
const (
	// solanaEventLogPrefix Solana 合约通过 msg! 宏输出的事件日志前缀
	solanaEventLogPrefix = "Program log: "

	// solanaProgramDataPrefix Anchor 框架 emit! 宏输出的事件数据前缀
	solanaProgramDataPrefix = "Program data: "
)

// SolanaEventData Solana 事件原始数据结构
// 由 blockchain-interactive-service 的 SolanaClient.processAndPublishEvent 发布
type SolanaEventData struct {
	Signature string        `json:"signature"`
	Slot      uint64        `json:"slot"`
	Logs      []string      `json:"logs"`
	Err       interface{}   `json:"err"`
	BlockTime int64         `json:"blockTime,omitempty"`
}

// SolanaAdapter Solana 链适配器
type SolanaAdapter struct{}

// NewSolanaAdapter 创建 Solana 适配器实例
func NewSolanaAdapter() *SolanaAdapter {
	return &SolanaAdapter{}
}

// ChainType 返回链类型标识
func (a *SolanaAdapter) ChainType() string {
	return strings.ToLower(chainPb.ChainType_Solana.String())
}

// ParseEvent 解析 Solana 事件数据
// 流程：JSON → SolanaEventData → 从 Logs 中提取事件 JSON → 解析为 Fields + RawData
func (a *SolanaAdapter) ParseEvent(eventData []byte, contractConfs []*chainPb.ContractDesc, contractName string) (*ParsedEvent, error) {
	// 1. 解析 Solana 事件结构
	var solanaEvent SolanaEventData
	if err := json.Unmarshal(eventData, &solanaEvent); err != nil {
		return nil, fmt.Errorf("%s solana event info: %v", code.ErrMsgJsonUnmarshal, err)
	}

	// 2. 从日志中提取事件数据 JSON
	eventJSON := extractEventJSONFromLogs(solanaEvent.Logs)
	if eventJSON == "" {
		return nil, fmt.Errorf("no event data found in solana transaction logs, signature: %s", solanaEvent.Signature)
	}

	// 3. 解析事件 JSON 为 map
	var fields map[string]interface{}
	if err := json.Unmarshal([]byte(eventJSON), &fields); err != nil {
		return nil, fmt.Errorf("%s solana event log data: %v", code.ErrMsgJsonUnmarshal, err)
	}

	// 4. 构造通用事件模型
	// 将 signature 和 slot 信息也加入 fields，便于追溯
	fields["_solana_signature"] = solanaEvent.Signature
	fields["_solana_slot"] = solanaEvent.Slot

	return &ParsedEvent{
		Fields:  fields,
		RawData: []byte(eventJSON),
	}, nil
}

// extractEventJSONFromLogs 从 Solana 交易日志中提取事件 JSON 数据
// Solana 合约通过 msg! 宏输出 JSON 格式的事件数据，格式为 "Program log: {json}"
// 优先查找最后一条包含有效 JSON 的日志（通常是业务事件数据）
func extractEventJSONFromLogs(logs []string) string {
	var lastValidJSON string

	for _, log := range logs {
		var content string

		// 尝试提取 "Program log: " 前缀的日志
		if strings.HasPrefix(log, solanaEventLogPrefix) {
			content = strings.TrimPrefix(log, solanaEventLogPrefix)
		} else if strings.HasPrefix(log, solanaProgramDataPrefix) {
			// Anchor emit! 格式暂不处理 base64 解码，跳过
			continue
		} else {
			continue
		}

		// 检查内容是否为有效 JSON 对象
		content = strings.TrimSpace(content)
		if len(content) > 0 && content[0] == '{' {
			// 验证是否为有效 JSON
			var temp map[string]interface{}
			if json.Unmarshal([]byte(content), &temp) == nil {
				lastValidJSON = content
			}
		}
	}

	return lastValidJSON
}

func init() {
	RegisterAdapter(NewSolanaAdapter())
}
