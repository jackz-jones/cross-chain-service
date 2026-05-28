package message

import (
	"fmt"
	"strings"

	"github.com/jackz-jones/cross-chain-service/internal/config"
	"github.com/zeromicro/go-zero/core/logx"
)

// MessageRouter 通用消息路由引擎
// 支持事件级别 → 合约级别 → 链级别的优先级路由查找
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
		rules:  rules,
		logger: logger,
		policy: unroutedPolicy,
	}
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

// findEventLevelRoutes 查找事件级别路由
func (r *MessageRouter) findEventLevelRoutes(msg *CrossChainMessage) []RouteTarget {
	var targets []RouteTarget
	for _, rule := range r.rules {
		if rule.Level != config.RouteLevelEvent {
			continue
		}
		if rule.SourceChain == msg.SourceChain &&
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
		if rule.SourceChain == msg.SourceChain &&
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
		if rule.SourceChain == msg.SourceChain {
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
