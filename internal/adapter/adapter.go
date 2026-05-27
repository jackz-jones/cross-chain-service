// Package adapter 链适配器接口定义
// 提供统一的链事件解析抽象，支持不同链类型的插件化接入
package adapter

import (
	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
)

// ChainAdapter 链适配器接口
// 每种链类型（Ethereum、Chainmaker、Fabric 等）需实现此接口
// 负责将链上原始事件数据解析为通用事件模型
type ChainAdapter interface {
	// ChainType 返回该适配器支持的链类型标识（小写）
	ChainType() string

	// ParseEvent 解析链上事件数据
	// eventData: 原始事件数据（JSON 格式）
	// contractConfs: 当前链下的合约配置（包含 ABI 等信息）
	// contractName: 触发事件的合约名称
	// 返回通用事件模型 ParsedEvent
	ParseEvent(eventData []byte, contractConfs []*chainPb.ContractDesc, contractName string) (*ParsedEvent, error)
}
