// Package message 通用跨链消息协议定义
package message

import (
	"encoding/json"
	"fmt"

	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
)

// PayloadCodec 定义 CrossChainMessage.Payload 与目标链调用参数（KeyValuePair 列表）之间的编解码方式。
//
// 背景：CrossChainMessage.Payload 是 []byte，注释明确表示由应用层自行编解码，
// 框架层不应假设其具体结构。但底层目标链 SDK 需要 []*KeyValuePair 才能发起调用，
// 因此需要一个明确的编解码抽象由业务方在 GenericEventHandler 中选择或实现。
//
// 默认提供 KVPairsCodec：期望 Payload 直接是 []*KeyValuePair 的 JSON。业务方若希望
// 直接使用业务结构体作为 Payload，需实现此接口并在 GenericEventHandler 上通过
// PayloadCodecProvider 暴露。
type PayloadCodec interface {
	// Decode 将 Payload []byte 转换为目标链调用所需的 KV 列表。
	// 失败应返回可读错误。
	Decode(payload []byte) ([]*chainPb.KeyValuePair, error)
}

// PayloadCodecProvider 是一个可选接口。若 GenericEventHandler 实现了此接口，
// handler_adapter 将使用其返回的 codec 处理 Payload；否则使用默认 KVPairsCodec。
type PayloadCodecProvider interface {
	PayloadCodec() PayloadCodec
}

// KVPairsCodec 默认实现：Payload 必须是 []*chainPb.KeyValuePair 的 JSON 编码。
// 这是历史行为，保留以兼容旧的示例与集成。
type KVPairsCodec struct{}

// Decode 将 JSON 反序列化为 KeyValuePair 列表。
func (KVPairsCodec) Decode(payload []byte) ([]*chainPb.KeyValuePair, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	var kvs []*chainPb.KeyValuePair
	if err := json.Unmarshal(payload, &kvs); err != nil {
		return nil, fmt.Errorf(
			"decode payload as []*KeyValuePair JSON failed: %w; "+
				"业务方需将 Payload 编码为 KeyValuePair 列表 JSON，或在 GenericEventHandler 上实现 PayloadCodecProvider 提供自定义 codec",
			err,
		)
	}
	return kvs, nil
}

// defaultPayloadCodec 是所有未显式声明 codec 的 handler 使用的默认实现。
var defaultPayloadCodec PayloadCodec = KVPairsCodec{}

// ResolvePayloadCodec 根据 handler 是否实现 PayloadCodecProvider 决定使用哪种 codec。
// 若 handler 为 nil 或未实现，返回默认 KVPairsCodec。
func ResolvePayloadCodec(handler GenericEventHandler) PayloadCodec {
	if handler == nil {
		return defaultPayloadCodec
	}
	if p, ok := handler.(PayloadCodecProvider); ok {
		if codec := p.PayloadCodec(); codec != nil {
			return codec
		}
	}
	return defaultPayloadCodec
}

// EncodeKVPairs 是一个便捷助手：将 map[string]string 编码为 []*KeyValuePair 的 JSON。
// 便于业务方在 BuildMessage 中快速构造符合默认 codec 期望的 Payload。
//
// 用法：
//
//	payload, err := message.EncodeKVPairs(map[string]string{"tokenId": "1", "owner": "0xabc"})
//	msg := message.NewCrossChainMessage(..., payload)
func EncodeKVPairs(fields map[string]string) ([]byte, error) {
	kvs := make([]*chainPb.KeyValuePair, 0, len(fields))
	for k, v := range fields {
		kvs = append(kvs, &chainPb.KeyValuePair{Key: k, Value: []byte(v)})
	}
	data, err := json.Marshal(kvs)
	if err != nil {
		return nil, fmt.Errorf("marshal KeyValuePair list failed: %w", err)
	}
	return data, nil
}
