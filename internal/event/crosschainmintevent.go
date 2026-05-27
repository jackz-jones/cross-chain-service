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
	nftconst "github.com/jackz-jones/nft-contract-go/const"
	nftEvent "github.com/jackz-jones/nft-contract-go/event"
	nftTypes "github.com/jackz-jones/nft-contract-go/types"
	"github.com/zeromicro/go-zero/core/logx"
)

// CrossChainMintEventHandler CrossChainMintEvent 事件处理器
type CrossChainMintEventHandler struct {
	*BaseEventHandler
	executor CrossChainExecutor
}

// crossChainMintProcessor CrossChainMint 事件业务处理器
type crossChainMintProcessor struct {
	handler *CrossChainMintEventHandler
}

// NewCrossChainMintEventHandler 实例化 CrossChainMintEvent 通知事件处理器
func NewCrossChainMintEventHandler(logger logx.Logger, svcCtx *svc.ServiceContext,
	contractDesc []*chainPb.ContractDesc,
	crossTargetChainConf *chainCli.ChainAndContractName) *CrossChainMintEventHandler {

	h := &CrossChainMintEventHandler{}
	processor := &crossChainMintProcessor{handler: h}
	registry := adapter.GlobalRegistry()

	h.BaseEventHandler = NewBaseEventHandler(logger, svcCtx, registry, processor, contractDesc, crossTargetChainConf)
	h.executor = NewDefaultCrossChainExecutor(logger, svcCtx, crossTargetChainConf, processor.EventName())

	return h
}

// EventName 返回事件名称
func (p *crossChainMintProcessor) EventName() string {
	return nftEvent.CrossChainMintEvent
}

// RequiredEventDataLength Chainmaker CrossChainMint 事件长度为 1
func (p *crossChainMintProcessor) RequiredEventDataLength() int {
	return 1
}

// ProcessParsedEvent 处理解析后的事件
func (p *crossChainMintProcessor) ProcessParsedEvent(parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent) error {
	if parsedEvent.EventDataItems != nil {
		return p.processChainmaker(parsedEvent, originalEvent)
	}

	// Solana 链：RawData 是纯 JSON 字符串，字段值为字符串类型
	if strings.EqualFold(originalEvent.ChainType, "solana") {
		return p.processSolana(parsedEvent, originalEvent)
	}

	return p.processEthereum(parsedEvent, originalEvent)
}

// processEthereum 处理以太坊事件
func (p *crossChainMintProcessor) processEthereum(parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent) error {
	h := p.handler

	var ccme commonEvent.CrossChainMintEvent
	err := json.Unmarshal(parsedEvent.RawData, &ccme)
	if err != nil {
		h.logger.Errorf("[%s] %s eth event result bytes: %v, data: %s", h.eventName(),
			code.ErrMsgJsonUnmarshal, err, string(parsedEvent.RawData))
		return fmt.Errorf("%s: %v", code.ErrMsgJsonUnmarshal, err)
	}

	err = p.handlerCrossChain(originalEvent.ContractType, ccme.NFTInfo.ID, ccme.NFTInfo.NftOwner.String(),
		ccme.NFTInfo.Holder.String(), ccme.NFTInfo.Sender.String(),
		ethCommon.Hash(ccme.NFTInfo.TokenId).String(), int(nftTypes.CrossState_Success))
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgHandlerCrossChain, err)
		return fmt.Errorf("%s: %v", code.ErrMsgHandlerCrossChain, err)
	}

	return nil
}

// processSolana 处理 Solana 链事件
func (p *crossChainMintProcessor) processSolana(parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent) error {
	h := p.handler

	// 从 Fields 中提取字段
	id := parsedEvent.GetString("id")
	owner := parsedEvent.GetString("owner")
	holder := parsedEvent.GetString("holder")
	sender := parsedEvent.GetString("sender")
	tokenId := parsedEvent.GetString("tokenId")

	err := p.handlerCrossChain(originalEvent.ContractType, id, owner, holder, sender,
		tokenId, int(nftTypes.CrossState_Success))
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgHandlerCrossChain, err)
		return fmt.Errorf("%s: %v", code.ErrMsgHandlerCrossChain, err)
	}

	return nil
}

// processChainmaker 处理长安链事件
func (p *crossChainMintProcessor) processChainmaker(parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent) error {
	h := p.handler

	ni := nftTypes.NFTInfo{}
	err := json.Unmarshal([]byte(parsedEvent.EventDataItems[0]), &ni)
	if err != nil {
		h.logger.Errorf("[%s] %s fileInfo: %v, data: %s", h.eventName(),
			code.ErrMsgJsonUnmarshal, err, parsedEvent.EventDataItems[0])
		return fmt.Errorf("%s fileInfo: %v", code.ErrMsgJsonUnmarshal, err)
	}

	err = p.handlerCrossChain(originalEvent.ContractType, ni.ID, ni.Owner, ni.Holder, ni.Sender,
		ni.TokenId, int(nftTypes.CrossState_Success))
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgHandlerCrossChain, err)
		return fmt.Errorf("%s: %v", code.ErrMsgHandlerCrossChain, err)
	}

	return nil
}

func (p *crossChainMintProcessor) handlerCrossChain(contractType, id, owner, holder, sender,
	tokenId string, crossState int) error {
	h := p.handler

	err := CheckWhitelist([]string{strings.ToLower(owner), strings.ToLower(holder), strings.ToLower(sender)})
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgCheckWhitelist, err)
		return fmt.Errorf("%s: %v", code.ErrMsgCheckWhitelist, err)
	}

	err = NotifyFile(id, owner, int(nftTypes.MessageType_CrossChainMintEvent))
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgNotifyFile, err)
		return fmt.Errorf("%s: %v", code.ErrMsgNotifyFile, err)
	}

	targetContractName := h.GetTargetContractName(contractType)

	kvs, err := CreateUpdateCrossChainStatusKvs(tokenId, crossState)
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgCreateUpdateCrossChainStatusKvs, err)
		return fmt.Errorf("%s: %v", code.ErrMsgCreateUpdateCrossChainStatusKvs, err)
	}

	// CrossChainMint 不需要回调，直接执行
	_, err = h.executor.Execute(
		h.crossTargetChainConf.ChainName,
		targetContractName,
		nftconst.MethodUpdateCrossChainStatus,
		kvs,
	)

	return err
}
