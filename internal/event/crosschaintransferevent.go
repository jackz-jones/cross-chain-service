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

// CrossChainTransferEventHandler CrossChainTransferEvent 事件处理器
type CrossChainTransferEventHandler struct {
	*BaseEventHandler
	executor CrossChainExecutor
}

// crossChainTransferProcessor CrossChainTransfer 事件业务处理器
type crossChainTransferProcessor struct {
	handler *CrossChainTransferEventHandler
}

// NewCrossChainTransferEventHandler 实例化 CrossChainTransferEvent 通知事件处理器
func NewCrossChainTransferEventHandler(logger logx.Logger, svcCtx *svc.ServiceContext,
	contractDesc []*chainPb.ContractDesc,
	crossTargetChainConf *chainCli.ChainAndContractName) *CrossChainTransferEventHandler {

	h := &CrossChainTransferEventHandler{}
	processor := &crossChainTransferProcessor{handler: h}
	registry := adapter.GlobalRegistry()

	h.BaseEventHandler = NewBaseEventHandler(logger, svcCtx, registry, processor, contractDesc, crossTargetChainConf)
	h.executor = newCrossChainExecutor(logger, svcCtx, crossTargetChainConf, processor.EventName())

	return h
}

// EventName 返回事件名称
func (p *crossChainTransferProcessor) EventName() string {
	return nftEvent.CrossChainTransferEvent
}

// RequiredEventDataLength Chainmaker CrossChainTransfer 事件长度为 3
func (p *crossChainTransferProcessor) RequiredEventDataLength() int {
	return 3
}

// ProcessParsedEvent 处理解析后的事件
func (p *crossChainTransferProcessor) ProcessParsedEvent(
	parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent,
) error {
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
func (p *crossChainTransferProcessor) processEthereum(
	parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent,
) error {
	h := p.handler

	var ccte commonEvent.CrossChainTransferEvent
	err := json.Unmarshal(parsedEvent.RawData, &ccte)
	if err != nil {
		h.logger.Errorf("[%s] %s eth event result bytes: %v, data: %s", h.eventName(),
			code.ErrMsgJsonUnmarshal, err, string(parsedEvent.RawData))
		return fmt.Errorf("%s: %v", code.ErrMsgJsonUnmarshal, err)
	}

	err = p.handlerCrossChain(originalEvent.ContractType, ccte.NFTInfo.ID, ccte.NFTInfo.NftOwner.String(),
		ccte.NFTInfo.Holder.String(), ccte.NFTInfo.Sender.String(),
		ethCommon.Hash(ccte.NFTInfo.OriginHash).String(), ccte.NFTInfo.Data,
		ccte.From.String(), ccte.To.String(),
		ethCommon.Hash(ccte.NFTInfo.TokenId).String())
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgHandlerCrossChain, err)
		return fmt.Errorf("%s: %v", code.ErrMsgHandlerCrossChain, err)
	}

	return nil
}

// processSolana 处理 Solana 链事件
func (p *crossChainTransferProcessor) processSolana(
	parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent,
) error {
	h := p.handler

	// 从 Fields 中提取字段
	id := parsedEvent.GetString("id")
	owner := parsedEvent.GetString("owner")
	holder := parsedEvent.GetString("holder")
	sender := parsedEvent.GetString("sender")
	originHash := parsedEvent.GetString("originHash")
	data := parsedEvent.GetString("data")
	from := parsedEvent.GetString("from")
	to := parsedEvent.GetString("to")
	tokenId := parsedEvent.GetString("tokenId")

	err := p.handlerCrossChain(originalEvent.ContractType, id, owner, holder, sender, originHash, data, from, to, tokenId)
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgHandlerCrossChain, err)
		return fmt.Errorf("%s: %v", code.ErrMsgHandlerCrossChain, err)
	}

	return nil
}

// processChainmaker 处理长安链事件
func (p *crossChainTransferProcessor) processChainmaker(
	parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent,
) error {
	h := p.handler

	ni := nftTypes.NFTInfo{}
	err := json.Unmarshal([]byte(parsedEvent.EventDataItems[0]), &ni)
	if err != nil {
		h.logger.Errorf("[%s] %s fileInfo: %v, data: %s", h.eventName(),
			code.ErrMsgJsonUnmarshal, err, parsedEvent.EventDataItems[0])
		return fmt.Errorf("%s fileInfo: %v", code.ErrMsgJsonUnmarshal, err)
	}

	from := parsedEvent.EventDataItems[1]
	to := parsedEvent.EventDataItems[2]

	err = p.handlerCrossChain(originalEvent.ContractType, ni.ID, ni.Owner, ni.Holder, ni.Sender, ni.OriginHash,
		ni.Data, from, to, ni.TokenId)
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgHandlerCrossChain, err)
		return fmt.Errorf("%s: %v", code.ErrMsgHandlerCrossChain, err)
	}

	return nil
}

func (p *crossChainTransferProcessor) handlerCrossChain(contractType, id, owner, holder,
	sender, originHash, data, from, to, tokenId string) error {
	h := p.handler

	err := CheckWhitelist([]string{strings.ToLower(owner), strings.ToLower(holder), strings.ToLower(sender),
		strings.ToLower(from), strings.ToLower(to)})
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgCheckWhitelist, err)
		return fmt.Errorf("%s: %v", code.ErrMsgCheckWhitelist, err)
	}

	err = NotifyFile(id, sender, int(nftTypes.MessageType_CrossChainTransferEvent))
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgNotifyFile, err)
		return fmt.Errorf("%s: %v", code.ErrMsgNotifyFile, err)
	}

	targetContractName := h.GetTargetContractName(contractType)

	kvs, err := CreateCrossChainMintKvs(id, owner, holder, originHash, data)
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgCreateCrossChainMintKvs, err)
		return fmt.Errorf("%s: %v", code.ErrMsgCreateCrossChainMintKvs, err)
	}

	_, err = h.executor.ExecuteWithCallback(
		h.crossTargetChainConf.ChainName,
		targetContractName,
		nftconst.MethodCrossChainMint,
		kvs,
		nftconst.MethodUpdateCrossChainStatus,
		func(errMsg string) ([]*chainPb.KeyValuePair, error) {
			return CreateUpdateCrossChainStatusKvs(tokenId, int(nftTypes.CrossState_Failed))
		},
	)

	return err
}
