package event

import (
	"context"
	"errors"
	"fmt"
	"sync"

	chainCli "github.com/jackz-jones/blockchain-interactive-service/chaininteractive"
	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/jackz-jones/cross-chain-service/internal/adapter"
	"github.com/jackz-jones/cross-chain-service/internal/code"
	"github.com/jackz-jones/cross-chain-service/internal/message"
	"github.com/jackz-jones/cross-chain-service/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

// handlerAdapter 适配器：将 GenericEventHandler 适配为 event.handler 接口
type handlerAdapter struct {
	ctx       context.Context
	svcCtx    *svc.ServiceContext
	logger    logx.Logger
	processor message.GenericEventHandler
	registry  *adapter.Registry

	// 当前链下的合约配置
	contractConfs []*chainPb.ContractDesc

	// 跨链的目标链配置（支持多目标广播）
	// key 用 chainName 索引，便于路由命中后按目标链查找目标合约信息
	crossTargetChainConfs []*chainCli.ChainAndContractName
}

// eventName 实现 handler 接口
func (h *handlerAdapter) eventName() string {
	return h.processor.Name()
}

// handleEvent 实现 handler 接口
func (h *handlerAdapter) handleEvent(event commonEvent.TradeGuardEvent) error {
	h.logger.Infof("[%s] start to handle event (generic): %#v", h.eventName(), event)

	// 1. 事件名校验
	if h.eventName() != event.EventName {
		h.logger.Errorf("[%s] %s", h.eventName(), code.ErrMsgNotMyEvent)
		return errors.New(code.ErrMsgNotMyEvent)
	}

	// 2. 空数据校验
	if len(event.EventData) == 0 {
		h.logger.Errorf("[%s] %s", h.eventName(), code.ErrMsgEventDataEmpty)
		return errors.New(code.ErrMsgEventDataEmpty)
	}

	// 3. 通过注册表获取适配器
	chainAdapter, err := h.registry.Get(event.ChainType)
	if err != nil {
		h.logger.Errorf("[%s] %s: %s", h.eventName(), code.ErrMsgUnknownChainType, event.ChainType)
		return fmt.Errorf("%s: %s", code.ErrMsgUnknownChainType, event.ChainType)
	}

	// 4. 适配器解析事件
	parsedEvent, err := chainAdapter.ParseEvent(event.EventData, h.contractConfs, event.ContractName)
	if err != nil {
		h.logger.Errorf("[%s] parse event error: %v, data: %s", h.eventName(), err, string(event.EventData))
		return err
	}

	// 5. Chainmaker 事件数据长度校验
	requiredLen := h.processor.RequiredEventDataLength()
	if requiredLen > 0 && parsedEvent.EventDataItems != nil && len(parsedEvent.EventDataItems) != requiredLen {
		h.logger.Errorf("[%s] %s: expected %d items, got %d", h.eventName(),
			code.ErrMsgInvalidEventInfoData, requiredLen, len(parsedEvent.EventDataItems))
		return fmt.Errorf("%s: expected %d items, got %d",
			code.ErrMsgInvalidEventInfoData, requiredLen, len(parsedEvent.EventDataItems))
	}

	// 6. 构建跨链消息
	msg, err := h.processor.BuildMessage(parsedEvent, event)
	if err != nil {
		h.logger.Errorf("[%s] build message error: %v", h.eventName(), err)
		return err
	}

	// 消息为 nil 表示该事件不需要跨链处理（如 needCrossChain=false）
	if msg == nil {
		h.logger.Infof("[%s] event skipped (no cross-chain needed)", h.eventName())
		return nil
	}

	// 7. 通过路由引擎解析目标链列表：
	//    - 路由命中：按每个目标广播执行
	//    - 路由未命中且 msg.TargetChain 已由业务方填充：仍执行单目标
	//    - 都没有则按未命中策略处理
	targets := h.resolveTargets(msg)
	if len(targets) == 0 {
		h.logger.Errorf("[%s] no route target resolved for message %s", h.eventName(), msg.MessageID)
		return nil
	}

	// 8. 执行跨链交易（支持多目标 fan-out）
	return h.executeMessageToTargets(msg, targets)
}

// resolveTargets 依据路由引擎与消息自身信息解析目标链列表。
// 保留旧行为：msg.TargetChain 已由业务方显式指定时，作为兜底目标。
func (h *handlerAdapter) resolveTargets(msg *message.CrossChainMessage) []message.RouteTarget {
	if h.svcCtx.Router != nil {
		if router, ok := h.svcCtx.Router.(*message.MessageRouter); ok {
			routeTargets, routeErr := router.Route(msg)
			if routeErr != nil {
				h.logger.Errorf("[%s] route message error: %v", h.eventName(), routeErr)
				return nil
			}
			if len(routeTargets) > 0 {
				return routeTargets
			}
		}
	}

	// 兜底：路由未命中但 msg.TargetChain 已有值 → 单目标执行
	if msg.TargetChain != "" {
		return []message.RouteTarget{
			{
				TargetChain:    msg.TargetChain,
				TargetContract: msg.TargetContract,
				Method:         msg.Method,
			},
		}
	}
	return nil
}

// executeMessageToTargets 对每个目标链独立执行跨链消息。
// 单目标时直接同步执行；多目标时并发 fan-out 并收集错误。
func (h *handlerAdapter) executeMessageToTargets(
	msg *message.CrossChainMessage,
	targets []message.RouteTarget,
) error {
	if len(targets) == 1 {
		clone := *msg
		clone.TargetChain = targets[0].TargetChain
		clone.TargetContract = targets[0].TargetContract
		if targets[0].Method != "" {
			clone.Method = targets[0].Method
		}
		return h.executeMessage(&clone)
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	for _, t := range targets {
		target := t
		wg.Add(1)
		go func() {
			defer wg.Done()
			clone := *msg
			clone.TargetChain = target.TargetChain
			clone.TargetContract = target.TargetContract
			if target.Method != "" {
				clone.Method = target.Method
			}
			if err := h.executeMessage(&clone); err != nil {
				h.logger.Errorf("[%s] execute message %s to target %s failed: %v",
					h.eventName(), msg.MessageID, target.TargetChain, err)
				mu.Lock()
				errs = append(errs, fmt.Errorf("target %s: %w", target.TargetChain, err))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// executeMessage 执行跨链消息
func (h *handlerAdapter) executeMessage(msg *message.CrossChainMessage) error {
	// 通过 codec 抽象将 Payload 解码为目标链所需的 KV 列表。
	// 业务方可在 GenericEventHandler 上实现 PayloadCodecProvider 提供自定义 codec，
	// 未实现时使用默认 KVPairsCodec（Payload 必须是 []*KeyValuePair 的 JSON）。
	codec := message.ResolvePayloadCodec(h.processor)
	kvs, err := codec.Decode(msg.Payload)
	if err != nil {
		h.logger.Errorf("[%s] decode payload error: %v", h.eventName(), err)
		return fmt.Errorf("decode payload for event %q: %w", h.eventName(), err)
	}

	// 获取目标合约名称
	targetContractName := msg.TargetContract
	if targetContractName == "" {
		targetContractName = h.GetTargetContractName(msg.TargetChain, msg.SourceContract)
	}

	// 创建执行器（把 msg 传入，让执行器可以同步更新其 Status/TxID/ErrorMessage）
	executor := message.CreateReliableExecutor(h.svcCtx, h.logger, msg)

	if msg.HasCallback() {
		// 回调 payload 使用同一份 codec 解码，保持一致性
		callbackKvs, err := codec.Decode(msg.CallbackPayload)
		if err != nil {
			h.logger.Errorf("[%s] decode callback payload error: %v", h.eventName(), err)
			return fmt.Errorf("decode callback payload for event %q: %w", h.eventName(), err)
		}

		callbackMethod := msg.CallbackMethod
		callbackKvsBuilder := func(errMsg string) ([]*chainPb.KeyValuePair, error) {
			return callbackKvs, nil
		}

		_, err = executor.ExecuteWithCallback(
			h.ctx,
			msg.TargetChain,
			targetContractName,
			msg.Method,
			kvs,
			callbackMethod,
			callbackKvsBuilder,
		)
		return err
	}

	// 不带回调的执行
	_, err = executor.Execute(
		h.ctx,
		msg.TargetChain,
		targetContractName,
		msg.Method,
		kvs,
	)
	return err
}

// GetTargetContractName 根据目标链名称和源合约类型查找目标链上的合约名称。
// targetChain 为空或未匹配时，按顺序遍历所有已知目标链配置查找同类型合约（向后兼容）。
func (h *handlerAdapter) GetTargetContractName(targetChain, contractType string) string {
	// 优先按目标链定位
	if targetChain != "" {
		for _, chainConf := range h.crossTargetChainConfs {
			if chainConf == nil || chainConf.ChainName != targetChain {
				continue
			}
			for _, contractConf := range chainConf.ContractDescs {
				if contractConf.ContractType.String() == contractType {
					return contractConf.ContractName
				}
			}
		}
	}

	// 兜底：任意目标链上的同类型合约
	for _, chainConf := range h.crossTargetChainConfs {
		if chainConf == nil {
			continue
		}
		for _, contractConf := range chainConf.ContractDescs {
			if contractConf.ContractType.String() == contractType {
				return contractConf.ContractName
			}
		}
	}
	return ""
}
