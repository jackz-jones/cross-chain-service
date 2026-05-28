// Package event 合约事件处理
package event

import (
	"fmt"
	"strings"

	"chainmaker.org/chainmaker/common/v2/json"
	ethCommon "github.com/ethereum/go-ethereum/common"
	chainCli "github.com/jackz-jones/blockchain-interactive-service/chaininteractive"
	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/jackz-jones/cross-chain-service/internal/adapter"
	"github.com/jackz-jones/cross-chain-service/internal/code"
	"github.com/jackz-jones/cross-chain-service/internal/svc"
	notificationConst "github.com/jackz-jones/notification-contract-go/const"
	notificationEvent "github.com/jackz-jones/notification-contract-go/event"
	notificationTypes "github.com/jackz-jones/notification-contract-go/types"
	"github.com/jackz-jones/notification-contract-go/util"
	"github.com/zeromicro/go-zero/core/logx"
)

// EnterpriseNotifiedEventHandler 企业通知事件处理器
type EnterpriseNotifiedEventHandler struct {
	*BaseEventHandler
	executor CrossChainExecutor
}

// enterpriseNotifiedProcessor 企业通知事件业务处理器
type enterpriseNotifiedProcessor struct {
	handler *EnterpriseNotifiedEventHandler
}

// NewEnterpriseNotifiedEventHandler 实例化企业通知事件处理器
func NewEnterpriseNotifiedEventHandler(logger logx.Logger, svcCtx *svc.ServiceContext,
	contractDesc []*chainPb.ContractDesc,
	crossTargetChainConf *chainCli.ChainAndContractName) *EnterpriseNotifiedEventHandler {

	h := &EnterpriseNotifiedEventHandler{}

	processor := &enterpriseNotifiedProcessor{handler: h}

	// 使用全局注册表
	registry := adapter.GlobalRegistry()

	h.BaseEventHandler = NewBaseEventHandler(logger, svcCtx, registry, processor, contractDesc, crossTargetChainConf)
	h.executor = newCrossChainExecutor(logger, svcCtx, crossTargetChainConf, processor.EventName())

	return h
}

// EventName 返回事件名称
func (p *enterpriseNotifiedProcessor) EventName() string {
	return notificationEvent.EnterpriseNotifiedEvent
}

// RequiredEventDataLength Chainmaker 企业身份事件长度为 2
func (p *enterpriseNotifiedProcessor) RequiredEventDataLength() int {
	return 2
}

// ProcessParsedEvent 处理解析后的事件
func (p *enterpriseNotifiedProcessor) ProcessParsedEvent(
	parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent,
) error {
	// 根据链类型处理
	if parsedEvent.EventDataItems != nil {
		// Chainmaker 链
		return p.processChainmaker(parsedEvent, originalEvent)
	}

	// Solana 链：RawData 是纯 JSON 字符串，字段值为字符串类型，需要单独处理
	if strings.EqualFold(originalEvent.ChainType, "solana") {
		return p.processSolana(parsedEvent, originalEvent)
	}

	// Ethereum 链
	return p.processEthereum(parsedEvent, originalEvent)
}

// processEthereum 处理以太坊事件
func (p *enterpriseNotifiedProcessor) processEthereum(
	parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent,
) error {
	h := p.handler

	// json 反序列化成企业通知事件结构
	var ei commonEvent.EnterpriseNotifiedEvent
	err := json.Unmarshal(parsedEvent.RawData, &ei)
	if err != nil {
		h.logger.Errorf("[%s] %s eth event result bytes: %v, data: %s", h.eventName(),
			code.ErrMsgJsonUnmarshal, err, string(parsedEvent.RawData))
		return fmt.Errorf("%s: %v", code.ErrMsgJsonUnmarshal, err)
	}

	// 如果需要跨链
	if ei.NeedCrossChain {
		err = p.handlerCrossChain(originalEvent.ContractType, ei.EnterpriseInfo.ID,
			ethCommon.Hash(ei.EnterpriseInfo.OriginHash).String(), ei.EnterpriseInfo.EnterpriseAddress.String(),
			ei.EnterpriseInfo.Did, ei.EnterpriseInfo.Sender.String())
		if err != nil {
			h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgHandlerCrossChain, err)
			return fmt.Errorf("%s: %v", code.ErrMsgHandlerCrossChain, err)
		}
	}

	return nil
}

// processSolana 处理 Solana 链事件
// Solana 合约通过日志输出 JSON 格式的事件数据，字段值为字符串类型
func (p *enterpriseNotifiedProcessor) processSolana(
	parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent,
) error {
	h := p.handler

	// 从 Fields 中提取字段
	id := parsedEvent.GetString("id")
	originHash := parsedEvent.GetString("originHash")
	address := parsedEvent.GetString("address")
	did := parsedEvent.GetString("did")
	sender := parsedEvent.GetString("sender")
	needCrossChain := parsedEvent.GetBool("needCrossChain")

	// 如果需要跨链
	if needCrossChain {
		err := p.handlerCrossChain(originalEvent.ContractType, id, originHash, address, did, sender)
		if err != nil {
			h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgHandlerCrossChain, err)
			return fmt.Errorf("%s: %v", code.ErrMsgHandlerCrossChain, err)
		}
	}

	return nil
}

// processChainmaker 处理长安链事件
func (p *enterpriseNotifiedProcessor) processChainmaker(
	parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent,
) error {
	h := p.handler

	// 解析企业通知事件结构
	ei := notificationTypes.EnterpriseInfo{}
	err := json.Unmarshal([]byte(parsedEvent.EventDataItems[0]), &ei)
	if err != nil {
		h.logger.Errorf("[%s] %s enterpriseInfo: %v, data: %s", h.eventName(),
			code.ErrMsgJsonUnmarshal, err, parsedEvent.EventDataItems[0])
		return fmt.Errorf("%s enterpriseInfo: %v", code.ErrMsgJsonUnmarshal, err)
	}

	// 解析是否跨链
	needCrossChain := util.BytesToBool([]byte(parsedEvent.EventDataItems[1]))

	// 如果需要跨链
	if needCrossChain {
		err = p.handlerCrossChain(originalEvent.ContractType, ei.ID, ei.OriginHash, ei.Address, ei.Did, ei.Sender)
		if err != nil {
			h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgHandlerCrossChain, err)
			return fmt.Errorf("%s: %v", code.ErrMsgHandlerCrossChain, err)
		}
	}

	return nil
}

func (p *enterpriseNotifiedProcessor) handlerCrossChain(contractType, id, originHash,
	enterpriseAddress, did, sender string) error {
	h := p.handler

	// 监管服务里面检查白名单
	err := CheckWhitelist([]string{strings.ToLower(enterpriseAddress), strings.ToLower(sender)})
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgCheckWhitelist, err)
		return fmt.Errorf("%s: %v", code.ErrMsgCheckWhitelist, err)
	}

	// 通知文件服务获取源文件
	err = NotifyFile(id, sender, int(notificationTypes.MessageType_Enterprise_Create))
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgNotifyFile, err)
		return fmt.Errorf("%s: %v", code.ErrMsgNotifyFile, err)
	}

	// 解析跨链目标链上的合约配置名称
	targetContractName := h.GetTargetContractName(contractType)

	// 构造 kvs
	kvs, err := CreateNotifyEnterpriseInfoKvs(id, originHash, enterpriseAddress, did, false)
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgCreateNotifyEnterpriseInfoKvs, err)
		return fmt.Errorf("%s: %v", code.ErrMsgCreateNotifyEnterpriseInfoKvs, err)
	}

	// 发送跨链交易（带回调）
	_, err = h.executor.ExecuteWithCallback(
		h.crossTargetChainConf.ChainName,
		targetContractName,
		notificationConst.MethodNotifyEnterpriseInfo,
		kvs,
		notificationConst.MethodCallback,
		func(errMsg string) ([]*chainPb.KeyValuePair, error) {
			return CreateCallbackKvs(id, originHash, errMsg, 711000,
				int(notificationTypes.MessageType_Enterprise_Create))
		},
	)

	return err
}
