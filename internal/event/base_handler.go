// Package event 合约事件处理
package event

import (
	"errors"
	"fmt"
	"strings"

	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/jackz-jones/cross-chain-service/internal/adapter"
	"github.com/jackz-jones/cross-chain-service/internal/code"
	"github.com/jackz-jones/cross-chain-service/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"

	chainCli "github.com/jackz-jones/blockchain-interactive-service/chaininteractive"
)

// EventProcessor 事件业务处理接口
// 每个具体事件处理器只需实现此接口，专注于业务逻辑
type EventProcessor interface {
	// EventName 返回处理的事件名称
	EventName() string

	// RequiredEventDataLength 返回 Chainmaker 事件数据所需的最小长度
	// 返回 0 表示不检查长度
	RequiredEventDataLength() int

	// ProcessParsedEvent 处理解析后的通用事件
	// parsedEvent: 适配器解析后的通用事件模型
	// originalEvent: 原始事件（包含链名、合约类型等元数据）
	ProcessParsedEvent(parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent) error
}

// BaseEventHandler 基础事件处理器
// 封装通用的事件处理流程：事件名校验 → 空数据校验 → 链类型路由 → 适配器解析 → 业务处理
type BaseEventHandler struct {
	svcCtx    *svc.ServiceContext
	logger    logx.Logger
	registry  *adapter.Registry
	processor EventProcessor

	// 当前链下的合约配置
	contractConfs []*chainPb.ContractDesc

	// 跨链的目标链配置
	crossTargetChainConf *chainCli.ChainAndContractName
}

// NewBaseEventHandler 创建基础事件处理器
func NewBaseEventHandler(
	logger logx.Logger,
	svcCtx *svc.ServiceContext,
	registry *adapter.Registry,
	processor EventProcessor,
	contractConfs []*chainPb.ContractDesc,
	crossTargetChainConf *chainCli.ChainAndContractName,
) *BaseEventHandler {
	return &BaseEventHandler{
		svcCtx:               svcCtx,
		logger:               logger,
		registry:             registry,
		processor:            processor,
		contractConfs:        contractConfs,
		crossTargetChainConf: crossTargetChainConf,
	}
}

// eventName 返回事件名称（实现 handler 接口）
func (h *BaseEventHandler) eventName() string {
	return h.processor.EventName()
}

// handleEvent 统一事件处理流程（实现 handler 接口）
func (h *BaseEventHandler) handleEvent(event commonEvent.TradeGuardEvent) error {
	h.logger.Infof("[%s] start to handle event: %#v", h.eventName(), event)

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

	// 6. 委托给具体处理器处理业务逻辑
	return h.processor.ProcessParsedEvent(parsedEvent, event)
}

// GetTargetContractName 根据合约类型获取目标链合约名称
func (h *BaseEventHandler) GetTargetContractName(contractType string) string {
	for _, contractConf := range h.crossTargetChainConf.ContractDescs {
		if strings.EqualFold(contractType, contractConf.ContractType.String()) {
			return contractConf.ContractName
		}
	}
	return ""
}

// GetCrossTargetChainConf 获取跨链目标链配置
func (h *BaseEventHandler) GetCrossTargetChainConf() *chainCli.ChainAndContractName {
	return h.crossTargetChainConf
}

// GetSvcCtx 获取服务上下文
func (h *BaseEventHandler) GetSvcCtx() *svc.ServiceContext {
	return h.svcCtx
}

// GetLogger 获取日志器
func (h *BaseEventHandler) GetLogger() logx.Logger {
	return h.logger
}
