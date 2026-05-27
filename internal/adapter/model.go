package adapter

// ParsedEvent 通用事件模型
// 将不同链类型的事件数据统一为通用结构，供后续业务逻辑使用
type ParsedEvent struct {
	// EventName 事件名称
	EventName string

	// Fields 事件字段映射
	// key 为字段名，value 为字段值（支持各种类型）
	// 例如：{"id": "xxx", "originHash": "0x...", "needCrossChain": true}
	Fields map[string]interface{}

	// RawData 原始解析后的字节数据（JSON 格式）
	// 用于需要完整反序列化到特定结构体的场景
	RawData []byte

	// EventDataItems Chainmaker 链特有的事件数据项列表
	// 对于 Ethereum 链此字段为 nil
	EventDataItems []string
}

// GetString 从 Fields 中获取字符串值
func (e *ParsedEvent) GetString(key string) string {
	if v, ok := e.Fields[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetBool 从 Fields 中获取布尔值
func (e *ParsedEvent) GetBool(key string) bool {
	if v, ok := e.Fields[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// GetInt 从 Fields 中获取整数值
func (e *ParsedEvent) GetInt(key string) int {
	if v, ok := e.Fields[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case int64:
			return int(val)
		case float64:
			return int(val)
		}
	}
	return 0
}
