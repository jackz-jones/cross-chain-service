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
	nftconst "github.com/jackz-jones/nft-contract-go/const"
	nftEvent "github.com/jackz-jones/nft-contract-go/event"
	nftTypes "github.com/jackz-jones/nft-contract-go/types"
	"github.com/zeromicro/go-zero/core/logx"
)

// CrossChainTransferEventHandler CrossChainTransferEvent 事件处理器
type CrossChainTransferEventHandler struct {
	svcCtx *svc.ServiceContext
	logger logx.Logger

	// 当前链下的合约配置
	contractConfs []*chainPb.ContractDesc

	// 跨链的目标链配置
	crossTargetChainConf *chainCli.ChainAndContractName
}

// NewCrossChainTransferEventHandler 实例化 CrossChainTransferEvent 通知事件处理器
func NewCrossChainTransferEventHandler(logger logx.Logger, svcCtx *svc.ServiceContext,
	contractDesc []*chainPb.ContractDesc,
	crossTargetChainConf *chainCli.ChainAndContractName) *CrossChainTransferEventHandler {
	return &CrossChainTransferEventHandler{
		svcCtx:               svcCtx,
		logger:               logger,
		contractConfs:        contractDesc,
		crossTargetChainConf: crossTargetChainConf,
	}
}

// eventName CrossChainTransferEvent 事件
func (h *CrossChainTransferEventHandler) eventName() string {
	return nftEvent.CrossChainTransferEvent
}

// handleEvent 事件处理
func (h *CrossChainTransferEventHandler) handleEvent(event commonEvent.TradeGuardEvent) error {
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
			return fmt.Errorf("%s for chain %s contract %s", code.ErrMsgInvalidAbi,
				event.ChainName, event.ContractName)
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

		// json 反序列化成CrossChainTransferEvent通知事件结构
		var ccte commonEvent.CrossChainTransferEvent
		err = json.Unmarshal(resultBytes, &ccte)
		if err != nil {
			h.logger.Errorf("[%s] %s eth event result bytes: %v, data: %s", h.eventName(),
				code.ErrMsgJsonUnmarshal, err, string(resultBytes))
			return fmt.Errorf("%s: %v", code.ErrMsgJsonUnmarshal, err)
		}

		// 发送跨链 crossChainMint
		err = h.handlerCrossChain(event.ContractType, ccte.NFTInfo.ID, ccte.NFTInfo.NftOwner.String(),
			ccte.NFTInfo.Holder.String(), ccte.NFTInfo.Sender.String(),
			ethCommon.Hash(ccte.NFTInfo.OriginHash).String(), ccte.NFTInfo.Data,
			ccte.From.String(), ccte.To.String(),
			ethCommon.Hash(ccte.NFTInfo.TokenId).String())
		if err != nil {
			h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgHandlerCrossChain, err)
			return fmt.Errorf("%s: %v", code.ErrMsgHandlerCrossChain, err)
		}

	case strings.ToLower(chainPb.ChainType_Chainmaker.String()):

		// 解析长安链事件结构
		var eventInfo common.ContractEventInfo
		if err := json.Unmarshal(event.EventData, &eventInfo); err != nil {
			h.logger.Errorf("[%s] %s chainmaker event info: %v, data: %s", h.eventName(),
				code.ErrMsgJsonUnmarshal, err, string(event.EventData))
			return fmt.Errorf("%s chainmaker event info: %v", code.ErrMsgJsonUnmarshal, err)
		}

		// CrossChainTransferEvent通知事件长度为 3
		if len(eventInfo.EventData) != 3 {
			h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgInvalidEventInfoData, eventInfo.EventData)
			return fmt.Errorf(" %s", code.ErrMsgInvalidEventInfoData)
		}

		// 解析CrossChainTransferEvent通知事件结构
		ni := nftTypes.NFTInfo{}
		err := json.Unmarshal([]byte(eventInfo.EventData[0]), &ni)
		if err != nil {
			h.logger.Errorf("[%s] %s fileInfo: %v, data: %s", h.eventName(),
				code.ErrMsgJsonUnmarshal, err, eventInfo.EventData[0])
			return fmt.Errorf("%s fileInfo: %v", code.ErrMsgJsonUnmarshal, err)
		}

		// 解析 from、to
		from := eventInfo.EventData[1]
		to := eventInfo.EventData[2]

		// 发送跨链 crossChainMint
		err = h.handlerCrossChain(event.ContractType, ni.ID, ni.Owner, ni.Holder, ni.Sender, ni.OriginHash,
			ni.Data, from, to, ni.TokenId)
		if err != nil {
			h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgHandlerCrossChain, err)
			return fmt.Errorf("%s: %v", code.ErrMsgHandlerCrossChain, err)
		}

	default:
		h.logger.Errorf("[%s] %s: %s", h.eventName(), code.ErrMsgUnknownChainType, event.ChainType)
		return fmt.Errorf("%s: %s", code.ErrMsgUnknownChainType, event.ChainType)
	}

	return nil
}

func (h *CrossChainTransferEventHandler) handlerCrossChain(contractType, id, owner, holder,
	sender, originHash, data, from, to, tokenId string) error {

	// 监管服务里面检查白名单
	err := CheckWhitelist([]string{strings.ToLower(owner), strings.ToLower(holder), strings.ToLower(sender),
		strings.ToLower(from), strings.ToLower(to)})
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgCheckWhitelist, err)
		return fmt.Errorf("%s: %v", code.ErrMsgCheckWhitelist, err)
	}

	// 通知文件服务获取源文件
	err = NotifyFile(id, sender, int(nftTypes.MessageType_CrossChainTransferEvent))
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
	kvs, err := CreateCrossChainMintKvs(id, owner, holder, originHash, data)
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgCreateCrossChainMintKvs, err)
		return fmt.Errorf("%s: %v", code.ErrMsgCreateCrossChainMintKvs, err)
	}

	// 发送 CrossChainMint 跨链交易
	crossChainMintTxId, err := SendCrossChainTx(h.crossTargetChainConf.ChainName, targetContractName,
		nftconst.MethodCrossChainMint, kvs, chainPb.MethodType_Invoke,
		h.svcCtx.Config.SendTxConf.WithSyncResult, h.svcCtx.Config.SendTxConf.TxTimeout,
		h.svcCtx.ChainInteractiveServiceClient)
	if err != nil {

		// 如果 CrossChainMint 失败，需要 UpdateCrossChainStatus 通知发起端失败
		updateCrossChainStatusKvs, err2 := CreateUpdateCrossChainStatusKvs(tokenId, int(nftTypes.CrossState_Failed))
		if err2 != nil {
			h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgCreateUpdateCrossChainStatusKvs, err2)
		} else {

			// 发送跨链 UpdateCrossChainStatus 交易
			updateCrossChainStatusTxId, err3 := SendCrossChainTx(h.crossTargetChainConf.ChainName, targetContractName,
				nftconst.MethodUpdateCrossChainStatus, updateCrossChainStatusKvs, chainPb.MethodType_Invoke,
				h.svcCtx.Config.SendTxConf.WithSyncResult, h.svcCtx.Config.SendTxConf.TxTimeout,
				h.svcCtx.ChainInteractiveServiceClient)
			if err3 != nil {
				h.logger.Errorf("[%s] %s UpdateCrossChainStatus: %v", h.eventName(), code.ErrMsgSendCrossChainTx, err3)
			} else {
				h.logger.Infof("[%s] %s UpdateCrossChainStatus: %s", h.eventName(),
					code.MsgSuccessToSendCrossChainTx, updateCrossChainStatusTxId)
			}
		}

		h.logger.Errorf("[%s] %s CrossChainMint: %v", h.eventName(), code.ErrMsgSendCrossChainTx, err)
		return fmt.Errorf("%s CrossChainMint: %v", code.ErrMsgSendCrossChainTx, err)
	}

	h.logger.Infof("[%s] %s CrossChainMint: %s", h.eventName(), code.MsgSuccessToSendCrossChainTx, crossChainMintTxId)
	return nil
}
