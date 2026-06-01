package event

import (
	"context"
	"errors"
	"fmt"

	"chainmaker.org/chainmaker/common/v2/json"
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

	// 跨链的目标链配置
	crossTargetChainConf *chainCli.ChainAndContractName
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
		return fmt.Errorf(" %s", code.ErrMsgInvalidEventInfoData)
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

	// 通过路由引擎填充目标链信息
	if h.svcCtx.Router != nil {
		if router, ok := h.svcCtx.Router.(*message.MessageRouter); ok {
			targets, routeErr := router.Route(msg)
			if routeErr != nil {
				h.logger.Errorf("[%s] route message error: %v", h.eventName(), routeErr)
				return routeErr
			}
			if len(targets) > 0 {
				msg.TargetChain = targets[0].TargetChain
				msg.TargetContract = targets[0].TargetContract
			}
		}
	}

	// 8. 执行跨链交易
	return h.executeMessage(msg)
}

// executeMessage 执行跨链消息
func (h *handlerAdapter) executeMessage(msg *message.CrossChainMessage) error {
	// 从 Payload 反序列化出 kvs
	var kvs []*chainPb.KeyValuePair
	if err := json.Unmarshal(msg.Payload, &kvs); err != nil {
		return fmt.Errorf("unmarshal payload to kvs: %w", err)
	}

	// 获取目标合约名称
	targetContractName := msg.TargetContract
	if targetContractName == "" {
		targetContractName = h.GetTargetContractName(msg.SourceContract)
	}

	// 创建执行器
	executor := message.CreateReliableExecutor(h.svcCtx, h.logger, msg.MessageID)

	if msg.HasCallback() {
		// 带回调的执行
		var callbackKvs []*chainPb.KeyValuePair
		if err := json.Unmarshal(msg.CallbackPayload, &callbackKvs); err != nil {
			h.logger.Errorf("[%s] unmarshal callback payload error: %v", h.eventName(), err)
			return fmt.Errorf("unmarshal callback payload: %w", err)
		}

		callbackMethod := msg.CallbackMethod
		callbackKvsBuilder := func(errMsg string) ([]*chainPb.KeyValuePair, error) {
			return callbackKvs, nil
		}

		_, err := executor.ExecuteWithCallback(
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
	_, err := executor.Execute(
		h.ctx,
		msg.TargetChain,
		targetContractName,
		msg.Method,
		kvs,
	)
	return err
}

// GetTargetContractName 根据合约类型获取目标链合约名称
func (h *handlerAdapter) GetTargetContractName(contractType string) string {
	for _, contractConf := range h.crossTargetChainConf.ContractDescs {
		if contractConf.ContractType.String() == contractType {
			return contractConf.ContractName
		}
	}
	return ""
}
