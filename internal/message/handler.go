// Package message 通用跨链消息协议定义
package message

import (
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/jackz-jones/cross-chain-service/internal/adapter"
)

// GenericEventHandler 通用事件处理器接口
// 替代现有的各硬编码 Handler，实现可插拔的事件处理
type GenericEventHandler interface {
	// Name 返回处理器名称（用于注册和路由匹配，通常为事件名称）
	Name() string

	// RequiredEventDataLength 返回 Chainmaker 事件数据所需的最小长度
	// 返回 0 表示不检查长度
	RequiredEventDataLength() int

	// BuildMessage 从解析后的事件构建跨链消息
	// parsedEvent: 适配器解析后的通用事件模型
	// originalEvent: 原始事件（包含链名、合约类型等元数据）
	BuildMessage(parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent) (*CrossChainMessage, error)
}
