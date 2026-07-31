package message

import (
	"fmt"
	"strings"

	"github.com/jackz-jones/cross-chain-service/internal/config"
	"github.com/zeromicro/go-zero/core/logx"
)

// MessageRouter 通用消息路由引擎
// 支持事件级别 → 合约级别 → 链级别的优先级路由查找
//
// 匹配策略（大小写不敏感，统一使用 strings.EqualFold）：
//   - SourceChain / TargetChain / SourceContractType / SourceEventName 均不区分大小写
//   - SourceChain 为空字符串时视为通配，匹配任意源链
//   - SourceContractType / SourceEventName 为空时表示"不比较该字段"
type MessageRouter struct { //nolint:revive
	rules  []config.DetailedRouteRule
	logger logx.Logger
	policy string // 路由未找到策略：discard / retry
}

// NewMessageRouter 创建消息路由引擎
func NewMessageRouter(rules []config.DetailedRouteRule, logger logx.Logger, unroutedPolicy string) *MessageRouter {
	if unroutedPolicy == "" {
		unroutedPolicy = "discard"
	}
	return &MessageRouter{
		rules:  NormalizeRules(rules),
		logger: logger,
		policy: unroutedPolicy,
	}
}

// NormalizeRules 对路由规则做启动期规范化：
// 仅做 TrimSpace，保留原大小写（比较阶段统一使用大小写不敏感匹配）。
// 独立导出便于测试与热加载复用。
func NormalizeRules(rules []config.DetailedRouteRule) []config.DetailedRouteRule {
	normalized := make([]config.DetailedRouteRule, len(rules))
	for i, r := range rules {
		normalized[i] = config.DetailedRouteRule{
			Level:              r.Level,
			SourceChain:        strings.TrimSpace(r.SourceChain),
			SourceContractType: strings.TrimSpace(r.SourceContractType),
			SourceEventName:    strings.TrimSpace(r.SourceEventName),
			TargetChain:        strings.TrimSpace(r.TargetChain),
			TargetContractType: strings.TrimSpace(r.TargetContractType),
			TargetMethod:       strings.TrimSpace(r.TargetMethod),
		}
	}
	return normalized
}

// Route 根据消息内容查找路由目标
// 查找优先级：事件级别 → 合约级别 → 链级别
// 支持广播到多条目标链
func (r *MessageRouter) Route(msg *CrossChainMessage) ([]RouteTarget, error) {
	// 按优先级查找路由
	// 1. 事件级别路由
	targets := r.findEventLevelRoutes(msg)
	if len(targets) > 0 {
		r.logger.Infof("[router] event-level route matched for message %s: %v", msg.MessageID, targets)
		return targets, nil
	}

	// 2. 合约级别路由
	targets = r.findContractLevelRoutes(msg)
	if len(targets) > 0 {
		r.logger.Infof("[router] contract-level route matched for message %s: %v", msg.MessageID, targets)
		return targets, nil
	}

	// 3. 链级别路由
	targets = r.findChainLevelRoutes(msg)
	if len(targets) > 0 {
		r.logger.Infof("[router] chain-level route matched for message %s: %v", msg.MessageID, targets)
		return targets, nil
	}

	// 路由未找到
	r.logger.Errorf("[router] no route found for message %s: sourceChain=%s, sourceContract=%s, method=%s",
		msg.MessageID, msg.SourceChain, msg.SourceContract, msg.Method)

	if r.policy == "retry" {
		return nil, fmt.Errorf("no route found for message %s (policy=retry)", msg.MessageID)
	}

	// 默认丢弃
	return nil, nil
}

// matchSourceChain 匹配源链：SourceChain 为空视为通配，否则大小写不敏感比较
func matchSourceChain(ruleChain, msgChain string) bool {
	if ruleChain == "" {
		return true
	}
	return strings.EqualFold(ruleChain, msgChain)
}

// findEventLevelRoutes 查找事件级别路由
func (r *MessageRouter) findEventLevelRoutes(msg *CrossChainMessage) []RouteTarget {
	var targets []RouteTarget
	for _, rule := range r.rules {
		if rule.Level != config.RouteLevelEvent {
			continue
		}
		if matchSourceChain(rule.SourceChain, msg.SourceChain) &&
			strings.EqualFold(rule.SourceContractType, msg.SourceContract) &&
			strings.EqualFold(rule.SourceEventName, msg.Method) {
			targets = append(targets, RouteTarget{
				TargetChain:    rule.TargetChain,
				TargetContract: rule.TargetContractType,
				Method:         rule.TargetMethod,
			})
		}
	}
	return targets
}

// findContractLevelRoutes 查找合约级别路由
func (r *MessageRouter) findContractLevelRoutes(msg *CrossChainMessage) []RouteTarget {
	var targets []RouteTarget
	for _, rule := range r.rules {
		if rule.Level != config.RouteLevelContract {
			continue
		}
		if matchSourceChain(rule.SourceChain, msg.SourceChain) &&
			strings.EqualFold(rule.SourceContractType, msg.SourceContract) {
			targets = append(targets, RouteTarget{
				TargetChain:    rule.TargetChain,
				TargetContract: rule.TargetContractType,
				Method:         msg.Method, // 合约级别路由保留消息原方法
			})
		}
	}
	return targets
}

// findChainLevelRoutes 查找链级别路由
func (r *MessageRouter) findChainLevelRoutes(msg *CrossChainMessage) []RouteTarget {
	var targets []RouteTarget
	for _, rule := range r.rules {
		if rule.Level != config.RouteLevelChain {
			continue
		}
		if matchSourceChain(rule.SourceChain, msg.SourceChain) {
			targets = append(targets, RouteTarget{
				TargetChain:    rule.TargetChain,
				TargetContract: msg.TargetContract, // 链级别路由保留消息原合约
				Method:         msg.Method,         // 链级别路由保留消息原方法
			})
		}
	}
	return targets
}

// GetPolicy 获取路由未找到策略
func (r *MessageRouter) GetPolicy() string {
	return r.policy
}
