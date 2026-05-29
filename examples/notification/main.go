//go:build ignore
// +build ignore

// Package main 展示如何基于通用跨链消息框架实现通知类跨链消息的 GenericEventHandler
//
// 本示例演示：
// 1. 如何实现企业身份通知的跨链处理器
// 2. 如何实现文件通知的跨链处理器
// 3. 如何处理 NeedCrossChain 标志位
// 4. 如何设置回调消息
package main

import (
	"encoding/json"
	"fmt"

	commonEvent "github.com/jackz-jones/common/event"
	"github.com/jackz-jones/cross-chain-service/internal/adapter"
	"github.com/jackz-jones/cross-chain-service/internal/message"
)

// ==================== 业务数据类型定义 ====================

// EnterpriseInfo 企业信息（对应 notification-contract-go/types.EnterpriseInfo）
type EnterpriseInfo struct {
	ID         string `json:"id"`
	OriginHash string `json:"originHash"`
	Address    string `json:"address"`
	Did        string `json:"did"`
}

// FileInfo 文件信息（对应 notification-contract-go/types.FileInfo）
type FileInfo struct {
	ID         string `json:"id"`
	OriginHash string `json:"originHash"`
	MsgType    int    `json:"msgType"`
}

// CallBackInfo 回调信息
type CallBackInfo struct {
	ID         string `json:"id"`
	OriginHash string `json:"originHash"`
	Code       int    `json:"code"`
	Msg        string `json:"msg"`
	MsgType    int    `json:"msgType"`
}

// EnterpriseNotifiedEvent 企业通知事件数据
type EnterpriseNotifiedEvent struct {
	ID             string `json:"id"`
	OriginHash     string `json:"originHash"`
	Address        string `json:"address"`
	Did            string `json:"did"`
	Sender         string `json:"sender"`
	NeedCrossChain bool   `json:"needCrossChain"`
}

// FileNotifiedEvent 文件通知事件数据
type FileNotifiedEvent struct {
	ID             string `json:"id"`
	OriginHash     string `json:"originHash"`
	Sender         string `json:"sender"`
	MsgType        int    `json:"msgType"`
	NeedCrossChain bool   `json:"needCrossChain"`
}

// ==================== 企业通知处理器 ====================

// EnterpriseNotifiedHandler 企业身份通知事件处理器
type EnterpriseNotifiedHandler struct{}

// Name 返回处理器名称
func (h *EnterpriseNotifiedHandler) Name() string {
	return "EnterpriseNotifiedEvent"
}

// RequiredEventDataLength Chainmaker 事件数据所需的最小长度
// 企业通知事件需要 5 个字段：id, originHash, address, did, needCrossChain
func (h *EnterpriseNotifiedHandler) RequiredEventDataLength() int {
	return 5
}

// BuildMessage 从解析后的事件构建跨链消息
func (h *EnterpriseNotifiedHandler) BuildMessage(
	parsedEvent *adapter.ParsedEvent,
	originalEvent commonEvent.TradeGuardEvent,
) (*message.CrossChainMessage, error) {

	var event EnterpriseNotifiedEvent

	// 优先使用 RawData（Solana/Ethereum 等链的 JSON 解析结果）
	if len(parsedEvent.RawData) > 0 {
		if err := json.Unmarshal(parsedEvent.RawData, &event); err != nil {
			return nil, fmt.Errorf("failed to parse enterprise notified event: %w", err)
		}
	} else if len(parsedEvent.EventDataItems) >= 5 {
		// Chainmaker 格式
		event = EnterpriseNotifiedEvent{
			ID:             parsedEvent.EventDataItems[0],
			OriginHash:     parsedEvent.EventDataItems[1],
			Address:        parsedEvent.EventDataItems[2],
			Did:            parsedEvent.EventDataItems[3],
			NeedCrossChain: parsedEvent.EventDataItems[4] == "true",
		}
	} else {
		return nil, fmt.Errorf("insufficient event data items: got %d, need 5", len(parsedEvent.EventDataItems))
	}

	// 如果不需要跨链，返回 nil 消息（框架会跳过处理）
	if !event.NeedCrossChain {
		return nil, nil
	}

	// 构建企业信息 payload
	enterpriseInfo := &EnterpriseInfo{
		ID:         event.ID,
		OriginHash: event.OriginHash,
		Address:    event.Address,
		Did:        event.Did,
	}
	payload, err := json.Marshal(enterpriseInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal enterprise info: %w", err)
	}

	// 创建跨链消息
	msg := message.NewCrossChainMessage(
		originalEvent.ChainName,
		"",
		originalEvent.ContractName,
		"",
		"NotifyEnterpriseInfo", // 目标合约方法
		payload,
	)

	// 设置回调：如果跨链失败，通知源链
	callbackInfo := &CallBackInfo{
		ID:         event.ID,
		OriginHash: event.OriginHash,
		Code:       711000,
		Msg:        "",
		MsgType:    1, // Enterprise_Create
	}
	callbackPayload, _ := json.Marshal(callbackInfo)
	msg.WithCallback("Callback", callbackPayload)

	// 设置业务元数据
	msg.WithMetadata("enterpriseId", event.ID)
	msg.WithMetadata("originHash", event.OriginHash)

	return msg, nil
}

// ==================== 文件通知处理器 ====================

// FileNotifiedHandler 文件通知事件处理器
type FileNotifiedHandler struct{}

// Name 返回处理器名称
func (h *FileNotifiedHandler) Name() string {
	return "FileNotifiedEvent"
}

// RequiredEventDataLength Chainmaker 事件数据所需的最小长度
func (h *FileNotifiedHandler) RequiredEventDataLength() int {
	return 4
}

// BuildMessage 从解析后的事件构建跨链消息
func (h *FileNotifiedHandler) BuildMessage(
	parsedEvent *adapter.ParsedEvent,
	originalEvent commonEvent.TradeGuardEvent,
) (*message.CrossChainMessage, error) {

	var event FileNotifiedEvent

	// 优先使用 RawData（Solana/Ethereum 等链的 JSON 解析结果）
	if len(parsedEvent.RawData) > 0 {
		if err := json.Unmarshal(parsedEvent.RawData, &event); err != nil {
			return nil, fmt.Errorf("failed to parse file notified event: %w", err)
		}
	} else if len(parsedEvent.EventDataItems) >= 4 {
		event = FileNotifiedEvent{
			ID:             parsedEvent.EventDataItems[0],
			OriginHash:     parsedEvent.EventDataItems[1],
			MsgType:        0, // 需要从字符串解析
			NeedCrossChain: parsedEvent.EventDataItems[3] == "true",
		}
	} else {
		return nil, fmt.Errorf("insufficient event data items: got %d, need 4", len(parsedEvent.EventDataItems))
	}

	if !event.NeedCrossChain {
		return nil, nil
	}

	fileInfo := &FileInfo{
		ID:         event.ID,
		OriginHash: event.OriginHash,
		MsgType:    event.MsgType,
	}
	payload, err := json.Marshal(fileInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal file info: %w", err)
	}

	msg := message.NewCrossChainMessage(
		originalEvent.ChainName,
		"",
		originalEvent.ContractName,
		"",
		"NotifyFileInfo",
		payload,
	)

	// 设置回调
	callbackInfo := &CallBackInfo{
		ID:         event.ID,
		OriginHash: event.OriginHash,
		Code:       712000,
		Msg:        "",
		MsgType:    event.MsgType,
	}
	callbackPayload, _ := json.Marshal(callbackInfo)
	msg.WithCallback("Callback", callbackPayload)

	msg.WithMetadata("fileId", event.ID)
	msg.WithMetadata("msgType", fmt.Sprintf("%d", event.MsgType))

	return msg, nil
}

// ==================== 注册示例 ====================

func main() {
	// 获取全局注册表
	registry := message.GlobalHandlerRegistry()

	// 注册通知类处理器
	registry.Register(&EnterpriseNotifiedHandler{})
	registry.Register(&FileNotifiedHandler{})

	fmt.Println("Notification cross-chain handlers registered successfully")
	fmt.Println("Registered handlers:")

	for name := range registry.GetAll() {
		fmt.Printf("  - %s\n", name)
	}
}
