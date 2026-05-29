// Package message 通用跨链消息协议定义
package message

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ProtocolVersion 当前协议版本
const ProtocolVersion = "1.0.0"

// MessageStatus 消息状态
type MessageStatus string //nolint:revive

const (
	// StatusPending 待处理
	StatusPending MessageStatus = "pending"
	// StatusSubmitted 已提交
	StatusSubmitted MessageStatus = "submitted"
	// StatusConfirmed 已确认
	StatusConfirmed MessageStatus = "confirmed"
	// StatusFailed 失败
	StatusFailed MessageStatus = "failed"
	// StatusTimeout 超时
	StatusTimeout MessageStatus = "timeout"
)

// CrossChainMessage 通用跨链消息
// 参考业内主流跨链协议（IBC、LayerZero、CCIP）的设计思想，定义与具体业务合约无关的通用消息格式
type CrossChainMessage struct {
	// MessageID 消息唯一标识（UUID），用于幂等去重
	MessageID string `json:"messageId"`

	// SourceChain 源链标识
	SourceChain string `json:"sourceChain"`

	// TargetChain 目标链标识
	TargetChain string `json:"targetChain"`

	// SourceContract 源合约地址/名称
	SourceContract string `json:"sourceContract"`

	// TargetContract 目标合约地址/名称
	TargetContract string `json:"targetContract"`

	// Method 目标合约调用方法名
	Method string `json:"method"`

	// Payload 通用消息体（[]byte），由应用层（插件）自行编解码，框架层不感知具体数据结构
	Payload []byte `json:"payload"`

	// CallbackMethod 回调方法名（可选，为空表示不需要回调）
	CallbackMethod string `json:"callbackMethod,omitempty"`

	// CallbackPayload 回调消息体（可选）
	CallbackPayload []byte `json:"callbackPayload,omitempty"`

	// Timeout 消息超时时间戳（可选，Unix 秒，0 表示不超时）
	Timeout int64 `json:"timeout,omitempty"`

	// Sequence 消息序列号（可选，用于有序传输）
	Sequence int64 `json:"sequence,omitempty"`

	// Version 协议版本号
	Version string `json:"version"`

	// CreatedAt 消息创建时间
	CreatedAt time.Time `json:"createdAt"`

	// Status 消息当前状态
	Status MessageStatus `json:"status"`

	// ErrorMessage 错误信息
	ErrorMessage string `json:"errorMessage,omitempty"`

	// TxID 跨链交易 ID（提交后填充）
	TxID string `json:"txId,omitempty"`

	// RetryCount 重试次数
	RetryCount int `json:"retryCount"`

	// Metadata 业务元数据（可选，用于插件传递业务特定信息）
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NewCrossChainMessage 创建新的跨链消息
func NewCrossChainMessage(
	sourceChain, targetChain, sourceContract, targetContract, method string,
	payload []byte,
) *CrossChainMessage {
	return &CrossChainMessage{
		MessageID:      uuid.New().String(),
		SourceChain:    sourceChain,
		TargetChain:    targetChain,
		SourceContract: sourceContract,
		TargetContract: targetContract,
		Method:         method,
		Payload:        payload,
		Version:        ProtocolVersion,
		CreatedAt:      time.Now(),
		Status:         StatusPending,
	}
}

// WithCallback 设置回调信息
func (m *CrossChainMessage) WithCallback(method string, payload []byte) *CrossChainMessage {
	m.CallbackMethod = method
	m.CallbackPayload = payload
	return m
}

// WithTimeout 设置超时时间
func (m *CrossChainMessage) WithTimeout(timeout time.Time) *CrossChainMessage {
	m.Timeout = timeout.Unix()
	return m
}

// WithSequence 设置序列号
func (m *CrossChainMessage) WithSequence(seq int64) *CrossChainMessage {
	m.Sequence = seq
	return m
}

// WithMetadata 设置业务元数据
func (m *CrossChainMessage) WithMetadata(key, value string) *CrossChainMessage {
	if m.Metadata == nil {
		m.Metadata = make(map[string]string)
	}
	m.Metadata[key] = value
	return m
}

// GetMetadata 获取业务元数据
func (m *CrossChainMessage) GetMetadata(key string) string {
	if m.Metadata == nil {
		return ""
	}
	return m.Metadata[key]
}

// IsExpired 检查消息是否已超时
func (m *CrossChainMessage) IsExpired() bool {
	if m.Timeout == 0 {
		return false
	}
	return time.Now().Unix() > m.Timeout
}

// HasCallback 是否有回调
func (m *CrossChainMessage) HasCallback() bool {
	return m.CallbackMethod != ""
}

// IsFireAndForget 是否为即发即忘消息（无回调、无超时）
func (m *CrossChainMessage) IsFireAndForget() bool {
	return !m.HasCallback() && m.Timeout == 0
}

// ToJSON 序列化为 JSON
func (m *CrossChainMessage) ToJSON() ([]byte, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cross chain message: %w", err)
	}
	return data, nil
}

// CrossChainMessageFromJSON 从 JSON 反序列化
func CrossChainMessageFromJSON(data []byte) (*CrossChainMessage, error) {
	var msg CrossChainMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cross chain message: %w", err)
	}
	return &msg, nil
}

// RouteTarget 路由目标
type RouteTarget struct {
	// TargetChain 目标链名称
	TargetChain string `json:"targetChain"`

	// TargetContract 目标合约名称
	TargetContract string `json:"targetContract"`

	// Method 目标方法名
	Method string `json:"method"`
}
