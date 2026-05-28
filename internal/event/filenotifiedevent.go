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

// FileNotifiedEventHandler 文件通知事件处理器
type FileNotifiedEventHandler struct {
	*BaseEventHandler
	executor CrossChainExecutor
}

// fileNotifiedProcessor 文件通知事件业务处理器
type fileNotifiedProcessor struct {
	handler *FileNotifiedEventHandler
}

// NewFileNotifiedEventHandler 实例化文件通知事件处理器
func NewFileNotifiedEventHandler(logger logx.Logger, svcCtx *svc.ServiceContext,
	contractDesc []*chainPb.ContractDesc,
	crossTargetChainConf *chainCli.ChainAndContractName) *FileNotifiedEventHandler {

	h := &FileNotifiedEventHandler{}
	processor := &fileNotifiedProcessor{handler: h}
	registry := adapter.GlobalRegistry()

	h.BaseEventHandler = NewBaseEventHandler(logger, svcCtx, registry, processor, contractDesc, crossTargetChainConf)
	h.executor = newCrossChainExecutor(logger, svcCtx, crossTargetChainConf, processor.EventName())

	return h
}

// EventName 返回事件名称
func (p *fileNotifiedProcessor) EventName() string {
	return notificationEvent.FileNotifiedEvent
}

// RequiredEventDataLength Chainmaker 文件通知事件长度为 2
func (p *fileNotifiedProcessor) RequiredEventDataLength() int {
	return 2
}

// ProcessParsedEvent 处理解析后的事件
func (p *fileNotifiedProcessor) ProcessParsedEvent(
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
func (p *fileNotifiedProcessor) processEthereum(
	parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent,
) error {
	h := p.handler

	var fi commonEvent.FileNotifiedEvent
	err := json.Unmarshal(parsedEvent.RawData, &fi)
	if err != nil {
		h.logger.Errorf("[%s] %s eth event result bytes: %v, data: %s", h.eventName(),
			code.ErrMsgJsonUnmarshal, err, string(parsedEvent.RawData))
		return fmt.Errorf("%s: %v", code.ErrMsgJsonUnmarshal, err)
	}

	if fi.NeedCrossChain {
		err = p.handlerCrossChain(originalEvent.ContractType, fi.FileInfo.ID,
			ethCommon.Hash(fi.FileInfo.OriginHash).String(),
			fi.FileInfo.Sender.String(), fi.FileInfo.MsgType)
		if err != nil {
			h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgHandlerCrossChain, err)
			return fmt.Errorf("%s: %v", code.ErrMsgHandlerCrossChain, err)
		}
	}

	return nil
}

// processSolana 处理 Solana 链事件
func (p *fileNotifiedProcessor) processSolana(
	parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent,
) error {
	h := p.handler

	// 从 Fields 中提取字段
	id := parsedEvent.GetString("id")
	originHash := parsedEvent.GetString("originHash")
	sender := parsedEvent.GetString("sender")
	msgType := parsedEvent.GetInt("msgType")
	needCrossChain := parsedEvent.GetBool("needCrossChain")

	// 如果需要跨链
	if needCrossChain {
		err := p.handlerCrossChain(originalEvent.ContractType, id, originHash, sender, msgType)
		if err != nil {
			h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgHandlerCrossChain, err)
			return fmt.Errorf("%s: %v", code.ErrMsgHandlerCrossChain, err)
		}
	}

	return nil
}

// processChainmaker 处理长安链事件
func (p *fileNotifiedProcessor) processChainmaker(
	parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent,
) error {
	h := p.handler

	fi := notificationTypes.FileInfo{}
	err := json.Unmarshal([]byte(parsedEvent.EventDataItems[0]), &fi)
	if err != nil {
		h.logger.Errorf("[%s] %s fileInfo: %v, data: %s", h.eventName(),
			code.ErrMsgJsonUnmarshal, err, parsedEvent.EventDataItems[0])
		return fmt.Errorf("%s fileInfo: %v", code.ErrMsgJsonUnmarshal, err)
	}

	needCrossChain := util.BytesToBool([]byte(parsedEvent.EventDataItems[1]))

	if needCrossChain {
		err = p.handlerCrossChain(originalEvent.ContractType, fi.ID, fi.OriginHash, fi.Sender, int(fi.MsgType))
		if err != nil {
			h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgHandlerCrossChain, err)
			return fmt.Errorf("%s: %v", code.ErrMsgHandlerCrossChain, err)
		}
	}

	return nil
}

func (p *fileNotifiedProcessor) handlerCrossChain(contractType, id, originHash, sender string, msgType int) error {
	h := p.handler

	err := CheckWhitelist([]string{strings.ToLower(sender)})
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgCheckWhitelist, err)
		return fmt.Errorf("%s: %v", code.ErrMsgCheckWhitelist, err)
	}

	err = NotifyFile(id, sender, msgType)
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgNotifyFile, err)
		return fmt.Errorf("%s: %v", code.ErrMsgNotifyFile, err)
	}

	targetContractName := h.GetTargetContractName(contractType)

	kvs, err := CreateNotifyFileInfoKvs(id, originHash, msgType, false)
	if err != nil {
		h.logger.Errorf("[%s] %s: %v", h.eventName(), code.ErrMsgCreateNotifyFileInfoKvs, err)
		return fmt.Errorf("%s: %v", code.ErrMsgCreateNotifyFileInfoKvs, err)
	}

	_, err = h.executor.ExecuteWithCallback(
		h.crossTargetChainConf.ChainName,
		targetContractName,
		notificationConst.MethodNotifyFileInfo,
		kvs,
		notificationConst.MethodCallback,
		func(errMsg string) ([]*chainPb.KeyValuePair, error) {
			return CreateCallbackKvs(id, originHash, errMsg, 712000, msgType)
		},
	)

	return err
}
