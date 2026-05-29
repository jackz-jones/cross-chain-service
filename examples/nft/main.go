//go:build ignore
// +build ignore

// Package main 展示如何基于通用跨链消息框架实现 NFT 跨链铸造/转移的 GenericEventHandler
//
// 本示例演示：
// 1. 如何实现 GenericEventHandler 接口
// 2. 如何从事件数据中解析 NFT 业务信息
// 3. 如何构建跨链消息（CrossChainMessage）
// 4. 如何注册处理器到全局注册表
package main

import (
	"encoding/json"
	"fmt"

	commonEvent "github.com/jackz-jones/common/event"
	"github.com/jackz-jones/cross-chain-service/internal/adapter"
	"github.com/jackz-jones/cross-chain-service/internal/message"
)

// ==================== 业务数据类型定义 ====================

// NFTInfo NFT 信息（对应 nft-contract-go/types.NFTInfo）
type NFTInfo struct {
	ID         string `json:"id"`
	Owner      string `json:"owner"`
	Holder     string `json:"holder"`
	OriginHash string `json:"originHash"`
	Data       string `json:"data"`
}

// CrossChainMintEvent 跨链铸造事件数据
type CrossChainMintEvent struct {
	TokenID    string `json:"tokenId"`
	Owner      string `json:"owner"`
	Holder     string `json:"holder"`
	OriginHash string `json:"originHash"`
	Data       string `json:"data"`
	Sender     string `json:"sender"`
}

// CrossChainTransferEvent 跨链转移事件数据
type CrossChainTransferEvent struct {
	TokenID    string `json:"tokenId"`
	From       string `json:"from"`
	To         string `json:"to"`
	OriginHash string `json:"originHash"`
	Sender     string `json:"sender"`
}

// ==================== NFT 跨链铸造处理器 ====================

// CrossChainMintHandler NFT 跨链铸造事件处理器
type CrossChainMintHandler struct{}

// Name 返回处理器名称
func (h *CrossChainMintHandler) Name() string {
	return "CrossChainMintEvent"
}

// RequiredEventDataLength Chainmaker 事件数据所需的最小长度
// NFT 铸造事件需要 5 个字段：tokenId, owner, holder, originHash, data
func (h *CrossChainMintHandler) RequiredEventDataLength() int {
	return 5
}

// BuildMessage 从解析后的事件构建跨链消息
func (h *CrossChainMintHandler) BuildMessage(
	parsedEvent *adapter.ParsedEvent,
	originalEvent commonEvent.TradeGuardEvent,
) (*message.CrossChainMessage, error) {

	// 从解析后的事件中提取 NFT 铸造信息
	var mintEvent CrossChainMintEvent

	// 优先使用 RawData（Solana/Ethereum 等链的 JSON 解析结果）
	if len(parsedEvent.RawData) > 0 {
		if err := json.Unmarshal(parsedEvent.RawData, &mintEvent); err != nil {
			return nil, fmt.Errorf("failed to parse mint event from RawData: %w", err)
		}
	} else if len(parsedEvent.EventDataItems) >= 5 {
		// Chainmaker 格式：按字段顺序解析
		mintEvent = CrossChainMintEvent{
			TokenID:    parsedEvent.EventDataItems[0],
			Owner:      parsedEvent.EventDataItems[1],
			Holder:     parsedEvent.EventDataItems[2],
			OriginHash: parsedEvent.EventDataItems[3],
			Data:       parsedEvent.EventDataItems[4],
		}
	} else {
		return nil, fmt.Errorf("insufficient event data items: got %d, need 5", len(parsedEvent.EventDataItems))
	}

	// 构建 NFT 信息作为 payload
	nftInfo := &NFTInfo{
		ID:         mintEvent.TokenID,
		Owner:      mintEvent.Owner,
		Holder:     mintEvent.Holder,
		OriginHash: mintEvent.OriginHash,
		Data:       mintEvent.Data,
	}
	payload, err := json.Marshal(nftInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal NFT info: %w", err)
	}

	// 创建跨链消息
	msg := message.NewCrossChainMessage(
		originalEvent.ChainName,    // 源链
		"",                         // 目标链（由路由表决定）
		originalEvent.ContractName, // 源合约
		"",                         // 目标合约（由路由表决定）
		"CrossChainMint",           // 目标方法
		payload,
	)

	// 设置业务元数据
	msg.WithMetadata("tokenId", mintEvent.TokenID)
	msg.WithMetadata("originHash", mintEvent.OriginHash)

	return msg, nil
}

// ==================== NFT 跨链转移处理器 ====================

// CrossChainTransferHandler NFT 跨链转移事件处理器
type CrossChainTransferHandler struct{}

// Name 返回处理器名称
func (h *CrossChainTransferHandler) Name() string {
	return "CrossChainTransferEvent"
}

// RequiredEventDataLength Chainmaker 事件数据所需的最小长度
func (h *CrossChainTransferHandler) RequiredEventDataLength() int {
	return 4
}

// BuildMessage 从解析后的事件构建跨链消息
func (h *CrossChainTransferHandler) BuildMessage(
	parsedEvent *adapter.ParsedEvent,
	originalEvent commonEvent.TradeGuardEvent,
) (*message.CrossChainMessage, error) {

	var transferEvent CrossChainTransferEvent

	// 优先使用 RawData（Solana/Ethereum 等链的 JSON 解析结果）
	if len(parsedEvent.RawData) > 0 {
		if err := json.Unmarshal(parsedEvent.RawData, &transferEvent); err != nil {
			return nil, fmt.Errorf("failed to parse transfer event from RawData: %w", err)
		}
	} else if len(parsedEvent.EventDataItems) >= 4 {
		transferEvent = CrossChainTransferEvent{
			TokenID:    parsedEvent.EventDataItems[0],
			From:       parsedEvent.EventDataItems[1],
			To:         parsedEvent.EventDataItems[2],
			OriginHash: parsedEvent.EventDataItems[3],
		}
	} else {
		return nil, fmt.Errorf("insufficient event data items: got %d, need 4", len(parsedEvent.EventDataItems))
	}

	payload, err := json.Marshal(transferEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transfer event: %w", err)
	}

	msg := message.NewCrossChainMessage(
		originalEvent.ChainName,
		"",
		originalEvent.ContractName,
		"",
		"CrossChainTransfer",
		payload,
	)

	msg.WithMetadata("tokenId", transferEvent.TokenID)
	msg.WithMetadata("from", transferEvent.From)
	msg.WithMetadata("to", transferEvent.To)

	return msg, nil
}

// ==================== 注册示例 ====================

func main() {
	// 获取全局注册表
	registry := message.GlobalHandlerRegistry()

	// 注册 NFT 跨链铸造处理器
	registry.Register(&CrossChainMintHandler{})

	// 注册 NFT 跨链转移处理器
	registry.Register(&CrossChainTransferHandler{})

	fmt.Println("NFT cross-chain handlers registered successfully")
	fmt.Println("Registered handlers:")

	for name := range registry.GetAll() {
		fmt.Printf("  - %s\n", name)
	}
}
