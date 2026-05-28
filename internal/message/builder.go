// Package message 通用跨链消息协议定义
package message

import (
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/jackz-jones/cross-chain-service/internal/adapter"
)

// MessageBuilder 消息构建器接口
// 由具体的事件处理器（插件）实现，将解析后的事件转换为通用跨链消息
type MessageBuilder interface { //nolint:revive
	// BuildMessage 从解析后的事件构建跨链消息
	// parsedEvent: 适配器解析后的通用事件模型
	// originalEvent: 原始事件（包含链名、合约类型等元数据）
	// 返回构建好的跨链消息和错误
	BuildMessage(parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent) (*CrossChainMessage, error)
}
