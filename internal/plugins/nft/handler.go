// Package nft NFT 跨链插件
// 将 nft-contract-go 的事件处理逻辑迁移为独立的 MessageBuilder 实现
package nft

import (
	"encoding/json"
	"fmt"
	"strings"

	ethCommon "github.com/ethereum/go-ethereum/common"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/jackz-jones/cross-chain-service/internal/adapter"
	"github.com/jackz-jones/cross-chain-service/internal/message"
	nftconst "github.com/jackz-jones/nft-contract-go/const"
	nftEvent "github.com/jackz-jones/nft-contract-go/event"
	nftTypes "github.com/jackz-jones/nft-contract-go/types"
)

// CrossChainMintHandler CrossChainMint 事件处理器
type CrossChainMintHandler struct{}

// NewCrossChainMintHandler 创建 CrossChainMint 事件处理器
func NewCrossChainMintHandler() *CrossChainMintHandler {
	return &CrossChainMintHandler{}
}

// Name 返回处理器名称
func (h *CrossChainMintHandler) Name() string {
	return "cross_chain_mint"
}

// RequiredEventDataLength 返回 Chainmaker 事件数据所需的最小长度
func (h *CrossChainMintHandler) RequiredEventDataLength() int {
	return 1
}

// BuildMessage 从解析后的事件构建跨链消息
func (h *CrossChainMintHandler) BuildMessage(
	parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent,
) (*message.CrossChainMessage, error) {
	// 根据链类型提取 NFT 信息
	var id, owner, holder, sender, tokenId string

	if parsedEvent.EventDataItems != nil {
		// Chainmaker 链
		ni := nftTypes.NFTInfo{}
		if err := json.Unmarshal([]byte(parsedEvent.EventDataItems[0]), &ni); err != nil {
			return nil, fmt.Errorf("unmarshal NFTInfo from chainmaker event: %w", err)
		}
		id = ni.ID
		owner = ni.Owner
		holder = ni.Holder
		sender = ni.Sender
		tokenId = ni.TokenId
	} else if strings.EqualFold(originalEvent.ChainType, "solana") {
		// Solana 链
		id = parsedEvent.GetString("id")
		owner = parsedEvent.GetString("owner")
		holder = parsedEvent.GetString("holder")
		sender = parsedEvent.GetString("sender")
		tokenId = parsedEvent.GetString("tokenId")
	} else {
		// Ethereum 链
		var ccme commonEvent.CrossChainMintEvent
		if err := json.Unmarshal(parsedEvent.RawData, &ccme); err != nil {
			return nil, fmt.Errorf("unmarshal CrossChainMintEvent from eth event: %w", err)
		}
		id = ccme.NFTInfo.ID
		owner = ccme.NFTInfo.NftOwner.String()
		holder = ccme.NFTInfo.Holder.String()
		sender = ccme.NFTInfo.Sender.String()
		tokenId = ethCommon.Hash(ccme.NFTInfo.TokenId).String()
	}

	// 白名单校验
	whitelistAddrs := []string{
		strings.ToLower(owner),
		strings.ToLower(holder),
		strings.ToLower(sender),
	}
	if err := checkWhitelist(whitelistAddrs); err != nil {
		return nil, fmt.Errorf("whitelist check failed: %w", err)
	}

	// 构造 UpdateCrossChainStatus 参数
	kvs, err := CreateUpdateCrossChainStatusKvs(tokenId, int(nftTypes.CrossState_Success))
	if err != nil {
		return nil, fmt.Errorf("create UpdateCrossChainStatus kvs: %w", err)
	}

	payload, err := json.Marshal(kvs)
	if err != nil {
		return nil, fmt.Errorf("marshal kvs: %w", err)
	}

	msg := message.NewCrossChainMessage(
		originalEvent.ChainName, "", // TargetChain 由路由填充
		originalEvent.ContractType, "", // TargetContract 由路由填充
		nftconst.MethodUpdateCrossChainStatus,
		payload,
	)

	// 保存业务元数据（原始代码中 id 用于 NotifyFile 通知）
	msg.WithMetadata("id", id)
	msg.WithMetadata("sender", sender)
	msg.WithMetadata("message_type", fmt.Sprintf("%d", int(nftTypes.MessageType_CrossChainMintEvent)))

	return msg, nil
}

// CrossChainTransferHandler CrossChainTransfer 事件处理器
type CrossChainTransferHandler struct{}

// NewCrossChainTransferHandler 创建 CrossChainTransfer 事件处理器
func NewCrossChainTransferHandler() *CrossChainTransferHandler {
	return &CrossChainTransferHandler{}
}

// Name 返回处理器名称
func (h *CrossChainTransferHandler) Name() string {
	return "cross_chain_transfer"
}

// RequiredEventDataLength 返回 Chainmaker 事件数据所需的最小长度
func (h *CrossChainTransferHandler) RequiredEventDataLength() int {
	return 3
}

// BuildMessage 从解析后的事件构建跨链消息
func (h *CrossChainTransferHandler) BuildMessage(
	parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent,
) (*message.CrossChainMessage, error) {
	// 根据链类型提取 NFT 信息
	var id, owner, holder, sender, originHash, data, from, to, tokenId string

	if parsedEvent.EventDataItems != nil {
		// Chainmaker 链
		ni := nftTypes.NFTInfo{}
		if err := json.Unmarshal([]byte(parsedEvent.EventDataItems[0]), &ni); err != nil {
			return nil, fmt.Errorf("unmarshal NFTInfo from chainmaker event: %w", err)
		}
		id = ni.ID
		owner = ni.Owner
		holder = ni.Holder
		sender = ni.Sender
		originHash = ni.OriginHash
		data = ni.Data
		tokenId = ni.TokenId
		from = parsedEvent.EventDataItems[1]
		to = parsedEvent.EventDataItems[2]
	} else if strings.EqualFold(originalEvent.ChainType, "solana") {
		// Solana 链
		id = parsedEvent.GetString("id")
		owner = parsedEvent.GetString("owner")
		holder = parsedEvent.GetString("holder")
		sender = parsedEvent.GetString("sender")
		originHash = parsedEvent.GetString("originHash")
		data = parsedEvent.GetString("data")
		from = parsedEvent.GetString("from")
		to = parsedEvent.GetString("to")
		tokenId = parsedEvent.GetString("tokenId")
	} else {
		// Ethereum 链
		var ccte commonEvent.CrossChainTransferEvent
		if err := json.Unmarshal(parsedEvent.RawData, &ccte); err != nil {
			return nil, fmt.Errorf("unmarshal CrossChainTransferEvent from eth event: %w", err)
		}
		id = ccte.NFTInfo.ID
		owner = ccte.NFTInfo.NftOwner.String()
		holder = ccte.NFTInfo.Holder.String()
		sender = ccte.NFTInfo.Sender.String()
		originHash = ethCommon.Hash(ccte.NFTInfo.OriginHash).String()
		data = ccte.NFTInfo.Data
		from = ccte.From.String()
		to = ccte.To.String()
		tokenId = ethCommon.Hash(ccte.NFTInfo.TokenId).String()
	}

	// 白名单校验
	if err := checkWhitelist([]string{
		strings.ToLower(owner), strings.ToLower(holder), strings.ToLower(sender),
		strings.ToLower(from), strings.ToLower(to),
	}); err != nil {
		return nil, fmt.Errorf("whitelist check failed: %w", err)
	}

	// 构造 CrossChainMint 参数
	kvs, err := CreateCrossChainMintKvs(id, owner, holder, originHash, data)
	if err != nil {
		return nil, fmt.Errorf("create CrossChainMint kvs: %w", err)
	}

	payload, err := json.Marshal(kvs)
	if err != nil {
		return nil, fmt.Errorf("marshal kvs: %w", err)
	}

	// 构造回调参数
	callbackKvs, err := CreateUpdateCrossChainStatusKvs(tokenId, int(nftTypes.CrossState_Failed))
	if err != nil {
		return nil, fmt.Errorf("create callback kvs: %w", err)
	}

	callbackPayload, err := json.Marshal(callbackKvs)
	if err != nil {
		return nil, fmt.Errorf("marshal callback kvs: %w", err)
	}

	msg := message.NewCrossChainMessage(
		originalEvent.ChainName, "", // TargetChain 由路由填充
		originalEvent.ContractType, "", // TargetContract 由路由填充
		nftconst.MethodCrossChainMint,
		payload,
	)
	msg.WithCallback(nftconst.MethodUpdateCrossChainStatus, callbackPayload)

	return msg, nil
}

// 事件名称常量（对应 nft-contract-go 中定义的事件名）
const (
	EventCrossChainMint     = nftEvent.CrossChainMintEvent
	EventCrossChainTransfer = nftEvent.CrossChainTransferEvent
)

// checkWhitelist 检查地址是否都在白名单（从 helper.go 迁移）
func checkWhitelist(addrList []string) error {
	// TODO：收集白名单
	whiteListMap := make(map[string]bool, 0)

	for _, addr := range addrList {
		if !whiteListMap[addr] {
			return fmt.Errorf("%s not in whitelist", addr)
		}
	}

	return nil
}

// Register 注册 NFT 插件的所有事件处理器
func Register() {
	message.RegisterHandler(NewCrossChainMintHandler())
	message.RegisterHandler(NewCrossChainTransferHandler())
}
