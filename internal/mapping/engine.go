// Package mapping 事件-动作映射规则引擎
package mapping

import (
	"fmt"
	"sync"
)

// Engine 规则引擎
// 管理事件映射规则，根据事件名查找对应的映射规则
type Engine struct {
	mu    sync.RWMutex
	rules map[string]*Rule
}

// NewEngine 创建规则引擎
func NewEngine() *Engine {
	return &Engine{
		rules: make(map[string]*Rule),
	}
}

// RegisterRule 注册映射规则
func (e *Engine) RegisterRule(rule *Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules[rule.SourceEventName] = rule
}

// RegisterRules 批量注册映射规则
func (e *Engine) RegisterRules(rules []*Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, rule := range rules {
		e.rules[rule.SourceEventName] = rule
	}
}

// GetRule 根据事件名获取映射规则
func (e *Engine) GetRule(eventName string) (*Rule, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rule, ok := e.rules[eventName]
	if !ok {
		return nil, fmt.Errorf("no mapping rule found for event: %s", eventName)
	}
	return rule, nil
}

// HasRule 检查是否存在指定事件的映射规则
func (e *Engine) HasRule(eventName string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.rules[eventName]
	return ok
}

// GetAllRules 获取所有规则
func (e *Engine) GetAllRules() []*Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rules := make([]*Rule, 0, len(e.rules))
	for _, rule := range e.rules {
		rules = append(rules, rule)
	}
	return rules
}

// MapFields 根据规则映射字段
// 将源事件字段按照 FieldMapping 转换为目标参数
func (e *Engine) MapFields(eventName string, sourceFields map[string]interface{}) (map[string]interface{}, error) {
	rule, err := e.GetRule(eventName)
	if err != nil {
		return nil, err
	}

	// 检查必需字段
	for _, field := range rule.RequiredFields {
		if _, ok := sourceFields[field]; !ok {
			return nil, fmt.Errorf("missing required field '%s' for event '%s'", field, eventName)
		}
	}

	// 如果没有字段映射，直接返回源字段
	if len(rule.FieldMapping) == 0 {
		return sourceFields, nil
	}

	// 执行字段映射
	result := make(map[string]interface{})
	for srcField, targetField := range rule.FieldMapping {
		if val, ok := sourceFields[srcField]; ok {
			result[targetField] = val
		}
	}

	return result, nil
}
