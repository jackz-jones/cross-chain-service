// Package mapping 事件-动作映射规则引擎
package mapping

// MappingRule 事件映射规则
// 定义源事件名到目标合约方法的映射关系
type MappingRule struct {
	// SourceEventName 源事件名称
	SourceEventName string `json:"sourceEventName"`

	// TargetMethod 目标合约方法名
	TargetMethod string `json:"targetMethod"`

	// CallbackMethod 失败回调方法名（可选）
	CallbackMethod string `json:"callbackMethod,omitempty"`

	// RequiredFields 必需的事件字段列表
	RequiredFields []string `json:"requiredFields,omitempty"`

	// FieldMapping 字段映射（源字段名 → 目标参数名）
	FieldMapping map[string]string `json:"fieldMapping,omitempty"`

	// Middlewares 中间件名称列表（按顺序执行）
	Middlewares []string `json:"middlewares,omitempty"`
}
