// Package event 合约事件处理
package event

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jackz-jones/cross-chain-service/internal/code"
	"github.com/jackz-jones/cross-chain-service/internal/svc"

	"chainmaker.org/chainmaker/common/v2/json"
	"chainmaker.org/chainmaker/pb-go/v2/common"
	ethCommon "github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	chainCli "github.com/jackz-jones/blockchain-interactive-service/chaininteractive"
	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	commonEvent "github.com/jackz-jones/common/event"
	notificationConst "github.com/jackz-jones/notification-contract-go/const"
	notificationEvent "github.com/jackz-jones/notification-contract-go/event"
	notificationTypes "github.com/jackz-jones/notification-contract-go/types"
	"github.com/jackz-jones/notification-contract-go/util"
	"github.com/zeromicro/go-zero/core/logx"
)

// EnterpriseNotifiedEventHandler 企业通知事件处理器
type EnterpriseNotifiedEventHandler struct {
	svcCtx *svc.ServiceContext
	logger logx.Logger

	// 当前链下的合约配置
	contractConfs []*chainPb.ContractDesc

	// 跨链的目标链配置
	crossTargetChainConf *chainCli.ChainAndContractName
}

// NewEnterpriseNotifiedEventHandler 实例化企业通知事件处理器
func NewEnterpriseNotifiedEventHandler(logger logx.Logger, svcCtx *svc.ServiceContext,
	contractDesc []*chainPb.ContractDesc,
	crossTargetChainConf *chainCli.ChainAndContractName) *EnterpriseNotifiedEventHandler {
	return &EnterpriseNotifiedEventHandler{
		svcCtx:               svcCtx,
		logger:               logger,
		contractConfs:        contractDesc,
		crossTargetChainConf: crossTargetChainConf,
	}
}

// eventName 企业通知事件
func (h *EnterpriseNotifiedEventHandler) eventName() string {
	return notificationEvent.EnterpriseNotifiedEvent
}

// handleEvent 事件处理
func (h *EnterpriseNotifiedEventHandler) handleEvent(event commonEvent.TradeGuardEvent) error {
	h.logger.Infof("[%s]start to handler event: %#v", h.eventName(), event)

	if h.eventName() != event.EventName {
		h.logger.Errorf("[%s] %s", h.eventName(), code.ErrMsgNotMyEvent)
		return errors.New(code.ErrMsgNotMyEvent)
	}

	// 事件数据异常
	if len(event.EventData) == 0 {
		h.logger.Error("[%s] %s", h.eventName(), code.ErrMsgEventDataEmpty)
		return errors.New(code.ErrMsgEventDataEmpty)
	}

	// 根据链类型解析事件
	switch strings.ToLower(event.ChainType) {
	case strings.ToLower(chainPb.ChainType_Ethereum.String()):

		// 解析以太坊事件结构
		var vLog ethTypes.Log
		if err := json.Unmarshal(event.EventData, &vLog); err != nil {
			h.logger.Errorf("[%s] %s ethereum event info: %v, data: %s", h.eventName(),
				code.ErrMsgJsonUnmarshal, err, string(event.EventData))
			return fmt.Errorf("%s ethereum event info: %v", code.ErrMsgJsonUnmarshal, err)
		}

		// 获取合约 abi
		abi := ""
		for _, contractConf := range h.contractConfs {
			if event.ContractName == contractConf.ContractName {
				abi = contractConf.Abi
			}
		}

		// 检查 abi
		if len(abi) == 0 {
			h.logger.Errorf("[%s] %s for chain %s contract %s", h.eventName(),
				code.ErrMsgInvalidAbi, event.ChainName, event.ContractName)
			return fmt.Errorf("%s for chain %s contract %s", code.ErrMsgInvalidAbi, event.ChainName, event.ContractName)
		}

		// 实例化 eth 事件处理器
		ethEventHandler, err := commonEvent.NewEthEventHandler(abi)
		if err != nil {
			h.logger.Errorf("[%s] %s: %v, data: %s", h.eventName(), code.ErrMsgNewEthEventHandler, err, string(event.EventData))
			return fmt.Errorf("%s: %v", code.ErrMsgNewEthEventHandler, err)
		}

		// 解析 eth 事件到 map[string]interface{}
		result, err := ethEventHandler.UnpackIntoMap(vLog)
		if err != nil {
			h.logger.Errorf("[%s] %s: %v, data: %#v", h.eventName(), code.ErrMsgUnpackIntoInterface, err, vLog)
			return fmt.Errorf("%s: %v", code.ErrMsgUnpackIntoInterface, err)
		}

		// json 序列化成字节
		resultBytes, err := json.Marshal(result)
		if err != nil {
			h.logger.Errorf("[%s] %s eth event result: %v, data: %#v", h.eventName(), code.ErrMsgJsonMarshal, err, result)
			return fmt.Errorf("%s: %v", code.ErrMsgJsonMarshal, err)
		}

		// json 反序列化成企业通知事件结构
		var ei commonEvent.EnterpriseNotifiedEvent
		err = json.Unmarshal(resultBytes, &ei)
		if err != nil {
			h.logger.Errorf("[%s] %s eth event result bytes: %v, data: %s", h.eventName(),
				code.ErrMsgJsonUnmarshal, err, string(resultBytes))
			return fmt.Errorf("%s: %v", code.ErrMsgJsonUnmarshal, err)
		}

		// 如果需要跨链，需要先去监管服务里面检查白名单，然后通知文件服务获取源文件，最后再发送到另一端链上
		if ei.NeedCrossChain {
			err = h.handlerCrossChain(event.ContractType, ei.EnterpriseInfo.ID,
				ethCommon.Hash(ei.EnterpriseInfo.OriginHash).String(), ei.EnterpriseInfo.EnterpriseAddress.String(),
				ei.EnterpriseInfo.Did, ei.EnterpriseInfo.Sender.String())
			if err != nil {
				h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgHandlerCrossChain, err)
				return fmt.Errorf("%s: %v", code.ErrMsgHandlerCrossChain, err)
			}
		}

	case strings.ToLower(chainPb.ChainType_Chainmaker.String()):

		// 解析长安链事件结构
		var eventInfo common.ContractEventInfo
		if err := json.Unmarshal(event.EventData, &eventInfo); err != nil {
			h.logger.Errorf("[%s] %s chainmaker event info: %v, data: %s", h.eventName(),
				code.ErrMsgJsonUnmarshal, err, string(event.EventData))
			return fmt.Errorf("%s chainmaker event info: %v", code.ErrMsgJsonUnmarshal, err)
		}

		// 企业身份事件长度为 2
		if len(eventInfo.EventData) != 2 {
			h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgInvalidEventInfoData, eventInfo.EventData)
			return fmt.Errorf(" %s", code.ErrMsgInvalidEventInfoData)
		}

		// 解析企业通知事件结构
		ei := notificationTypes.EnterpriseInfo{}
		err := json.Unmarshal([]byte(eventInfo.EventData[0]), &ei)
		if err != nil {
			h.logger.Errorf("[%s] %s enterpriseInfo: %v, data: %s", h.eventName(),
				code.ErrMsgJsonUnmarshal, err, eventInfo.EventData[0])
			return fmt.Errorf("%s enterpriseInfo: %v", code.ErrMsgJsonUnmarshal, err)
		}

		// 解析是否跨链
		needCrossChain := util.BytesToBool([]byte(eventInfo.EventData[1]))

		// 如果需要跨链，需要先去监管服务里面检查白名单，然后通知文件服务获取源文件，最后再发送到另一端链上
		if needCrossChain {
			err = h.handlerCrossChain(event.ContractType, ei.ID, ei.OriginHash, ei.Address, ei.Did, ei.Sender)
			if err != nil {
				h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgHandlerCrossChain, err)
				return fmt.Errorf("%s: %v", code.ErrMsgHandlerCrossChain, err)
			}
		}

	default:
		h.logger.Errorf("[%s] %s: %s", h.eventName(), code.ErrMsgUnknownChainType, event.ChainType)
		return fmt.Errorf("%s: %s", code.ErrMsgUnknownChainType, event.ChainType)
	}

	return nil
}

func (h *EnterpriseNotifiedEventHandler) handlerCrossChain(contractType, id, originHash,
	enterpriseAddress, did, sender string) error {

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
	targetContractName := ""
	for _, contractConf := range h.crossTargetChainConf.ContractDescs {
		if strings.EqualFold(contractType, contractConf.ContractType.String()) {
			targetContractName = contractConf.ContractName
		}
	}

	// 最后再发送到另一端链上
	kvs, err := CreateNotifyEnterpriseInfoKvs(id, originHash, enterpriseAddress, did, false)
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgCreateNotifyEnterpriseInfoKvs, err)
		return fmt.Errorf("%s: %v", code.ErrMsgCreateNotifyEnterpriseInfoKvs, err)
	}

	// 发送 NotifyEnterpriseInfo 跨链交易
	notifyEnterpriseInfoTxId, err := SendCrossChainTx(h.crossTargetChainConf.ChainName, targetContractName,
		notificationConst.MethodNotifyEnterpriseInfo, kvs, chainPb.MethodType_Invoke,
		h.svcCtx.Config.SendTxConf.WithSyncResult, h.svcCtx.Config.SendTxConf.TxTimeout,
		h.svcCtx.ChainInteractiveServiceClient)
	if err != nil {

		// 如果 NotifyEnterpriseInfo 失败，需要 callback 通知发起端失败
		callbackKvs, err2 := CreateCallbackKvs(id, originHash, err.Error(), 711000,
			int(notificationTypes.MessageType_Enterprise_Create))
		if err2 != nil {
			h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgCreateCallbackKvs, err2)
		} else {

			// 发送跨链 callback 交易
			callBackTxId, err3 := SendCrossChainTx(h.crossTargetChainConf.ChainName, targetContractName,
				notificationConst.MethodCallback, callbackKvs, chainPb.MethodType_Invoke,
				h.svcCtx.Config.SendTxConf.WithSyncResult, h.svcCtx.Config.SendTxConf.TxTimeout,
				h.svcCtx.ChainInteractiveServiceClient)
			if err3 != nil {
				h.logger.Errorf("[%s] %s Callback: %v", h.eventName(), code.ErrMsgSendCrossChainTx, err3)
			} else {
				h.logger.Infof("[%s] %s Callback: %s", h.eventName(), code.MsgSuccessToSendCrossChainTx, callBackTxId)
			}
		}

		h.logger.Errorf("[%s] %s NotifyEnterpriseInfo: %v", h.eventName(), code.ErrMsgSendCrossChainTx, err)
		return fmt.Errorf("%s NotifyEnterpriseInfo: %v", code.ErrMsgSendCrossChainTx, err)
	}

	h.logger.Infof("[%s] %s NotifyEnterpriseInfo: %s", h.eventName(),
		code.MsgSuccessToSendCrossChainTx, notifyEnterpriseInfoTxId)
	return nil
}
