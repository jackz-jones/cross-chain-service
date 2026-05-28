// Package notification Notification 跨链插件
// 将 notification-contract-go 的事件处理逻辑迁移为独立的 MessageBuilder 实现
package notification

import (
	"encoding/json"
	"fmt"
	"strings"

	ethCommon "github.com/ethereum/go-ethereum/common"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/jackz-jones/cross-chain-service/internal/adapter"
	"github.com/jackz-jones/cross-chain-service/internal/message"
	notificationConst "github.com/jackz-jones/notification-contract-go/const"
	notificationEvent "github.com/jackz-jones/notification-contract-go/event"
	notificationTypes "github.com/jackz-jones/notification-contract-go/types"
	"github.com/jackz-jones/notification-contract-go/util"
)

// EnterpriseNotifiedHandler 企业通知事件处理器
type EnterpriseNotifiedHandler struct{}

// NewEnterpriseNotifiedHandler 创建企业通知事件处理器
func NewEnterpriseNotifiedHandler() *EnterpriseNotifiedHandler {
	return &EnterpriseNotifiedHandler{}
}

// Name 返回处理器名称
func (h *EnterpriseNotifiedHandler) Name() string {
	return "enterprise_notified"
}

// RequiredEventDataLength 返回 Chainmaker 事件数据所需的最小长度
func (h *EnterpriseNotifiedHandler) RequiredEventDataLength() int {
	return 2
}

// BuildMessage 从解析后的事件构建跨链消息
func (h *EnterpriseNotifiedHandler) BuildMessage(
	parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent,
) (*message.CrossChainMessage, error) {
	// 根据链类型提取企业信息
	var id, originHash, enterpriseAddress, did, sender string
	var needCrossChain bool

	if parsedEvent.EventDataItems != nil {
		// Chainmaker 链
		ei := notificationTypes.EnterpriseInfo{}
		if err := json.Unmarshal([]byte(parsedEvent.EventDataItems[0]), &ei); err != nil {
			return nil, fmt.Errorf("unmarshal EnterpriseInfo from chainmaker event: %w", err)
		}
		id = ei.ID
		originHash = ei.OriginHash
		enterpriseAddress = ei.Address
		did = ei.Did
		sender = ei.Sender
		needCrossChain = util.BytesToBool([]byte(parsedEvent.EventDataItems[1]))
	} else if strings.EqualFold(originalEvent.ChainType, "solana") {
		// Solana 链
		id = parsedEvent.GetString("id")
		originHash = parsedEvent.GetString("originHash")
		enterpriseAddress = parsedEvent.GetString("address")
		did = parsedEvent.GetString("did")
		sender = parsedEvent.GetString("sender")
		needCrossChain = parsedEvent.GetBool("needCrossChain")
	} else {
		// Ethereum 链
		var ei commonEvent.EnterpriseNotifiedEvent
		if err := json.Unmarshal(parsedEvent.RawData, &ei); err != nil {
			return nil, fmt.Errorf("unmarshal EnterpriseNotifiedEvent from eth event: %w", err)
		}
		id = ei.EnterpriseInfo.ID
		originHash = ethCommon.Hash(ei.EnterpriseInfo.OriginHash).String()
		enterpriseAddress = ei.EnterpriseInfo.EnterpriseAddress.String()
		did = ei.EnterpriseInfo.Did
		sender = ei.EnterpriseInfo.Sender.String()
		needCrossChain = ei.NeedCrossChain
	}

	// 如果不需要跨链，返回 nil 表示跳过
	if !needCrossChain {
		return nil, nil
	}

	// 白名单校验
	if err := checkWhitelist([]string{strings.ToLower(enterpriseAddress), strings.ToLower(sender)}); err != nil {
		return nil, fmt.Errorf("whitelist check failed: %w", err)
	}

	// 构造 NotifyEnterpriseInfo 参数
	kvs, err := CreateNotifyEnterpriseInfoKvs(id, originHash, enterpriseAddress, did, false)
	if err != nil {
		return nil, fmt.Errorf("create NotifyEnterpriseInfo kvs: %w", err)
	}

	payload, err := json.Marshal(kvs)
	if err != nil {
		return nil, fmt.Errorf("marshal kvs: %w", err)
	}

	// 构造回调参数
	callbackKvs, err := CreateCallbackKvs(id, originHash, "", 711000, int(notificationTypes.MessageType_Enterprise_Create))
	if err != nil {
		return nil, fmt.Errorf("create callback kvs: %w", err)
	}

	callbackPayload, err := json.Marshal(callbackKvs)
	if err != nil {
		return nil, fmt.Errorf("marshal callback kvs: %w", err)
	}

	msg := message.NewCrossChainMessage(
		originalEvent.ChainName, "",
		originalEvent.ContractType, "",
		notificationConst.MethodNotifyEnterpriseInfo,
		payload,
	)
	msg.WithCallback(notificationConst.MethodCallback, callbackPayload)

	// 保存业务元数据
	msg.WithMetadata("id", id)
	msg.WithMetadata("sender", sender)
	msg.WithMetadata("message_type", fmt.Sprintf("%d", int(notificationTypes.MessageType_Enterprise_Create)))

	return msg, nil
}

// FileNotifiedHandler 文件通知事件处理器
type FileNotifiedHandler struct{}

// NewFileNotifiedHandler 创建文件通知事件处理器
func NewFileNotifiedHandler() *FileNotifiedHandler {
	return &FileNotifiedHandler{}
}

// Name 返回处理器名称
func (h *FileNotifiedHandler) Name() string {
	return "file_notified"
}

// RequiredEventDataLength 返回 Chainmaker 事件数据所需的最小长度
func (h *FileNotifiedHandler) RequiredEventDataLength() int {
	return 2
}

// BuildMessage 从解析后的事件构建跨链消息
func (h *FileNotifiedHandler) BuildMessage(
	parsedEvent *adapter.ParsedEvent, originalEvent commonEvent.TradeGuardEvent,
) (*message.CrossChainMessage, error) {
	// 根据链类型提取文件信息
	var id, originHash, sender string
	var msgType int
	var needCrossChain bool

	if parsedEvent.EventDataItems != nil {
		// Chainmaker 链
		fi := notificationTypes.FileInfo{}
		if err := json.Unmarshal([]byte(parsedEvent.EventDataItems[0]), &fi); err != nil {
			return nil, fmt.Errorf("unmarshal FileInfo from chainmaker event: %w", err)
		}
		id = fi.ID
		originHash = fi.OriginHash
		sender = fi.Sender
		msgType = int(fi.MsgType)
		needCrossChain = util.BytesToBool([]byte(parsedEvent.EventDataItems[1]))
	} else if strings.EqualFold(originalEvent.ChainType, "solana") {
		// Solana 链
		id = parsedEvent.GetString("id")
		originHash = parsedEvent.GetString("originHash")
		sender = parsedEvent.GetString("sender")
		msgType = parsedEvent.GetInt("msgType")
		needCrossChain = parsedEvent.GetBool("needCrossChain")
	} else {
		// Ethereum 链
		var fi commonEvent.FileNotifiedEvent
		if err := json.Unmarshal(parsedEvent.RawData, &fi); err != nil {
			return nil, fmt.Errorf("unmarshal FileNotifiedEvent from eth event: %w", err)
		}
		id = fi.FileInfo.ID
		originHash = ethCommon.Hash(fi.FileInfo.OriginHash).String()
		sender = fi.FileInfo.Sender.String()
		msgType = fi.FileInfo.MsgType
		needCrossChain = fi.NeedCrossChain
	}

	// 如果不需要跨链，返回 nil 表示跳过
	if !needCrossChain {
		return nil, nil
	}

	// 白名单校验
	if err := checkWhitelist([]string{strings.ToLower(sender)}); err != nil {
		return nil, fmt.Errorf("whitelist check failed: %w", err)
	}

	// 构造 NotifyFileInfo 参数
	kvs, err := CreateNotifyFileInfoKvs(id, originHash, msgType, false)
	if err != nil {
		return nil, fmt.Errorf("create NotifyFileInfo kvs: %w", err)
	}

	payload, err := json.Marshal(kvs)
	if err != nil {
		return nil, fmt.Errorf("marshal kvs: %w", err)
	}

	// 构造回调参数
	callbackKvs, err := CreateCallbackKvs(id, originHash, "", 712000, msgType)
	if err != nil {
		return nil, fmt.Errorf("create callback kvs: %w", err)
	}

	callbackPayload, err := json.Marshal(callbackKvs)
	if err != nil {
		return nil, fmt.Errorf("marshal callback kvs: %w", err)
	}

	msg := message.NewCrossChainMessage(
		originalEvent.ChainName, "",
		originalEvent.ContractType, "",
		notificationConst.MethodNotifyFileInfo,
		payload,
	)
	msg.WithCallback(notificationConst.MethodCallback, callbackPayload)

	// 保存业务元数据
	msg.WithMetadata("id", id)
	msg.WithMetadata("sender", sender)
	msg.WithMetadata("message_type", fmt.Sprintf("%d", msgType))

	return msg, nil
}

// 事件名称常量
const (
	EventEnterpriseNotified = notificationEvent.EnterpriseNotifiedEvent
	EventFileNotified       = notificationEvent.FileNotifiedEvent
)

// checkWhitelist 检查地址是否都在白名单
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

// Register 注册 Notification 插件的所有事件处理器
func Register() {
	message.RegisterHandler(NewEnterpriseNotifiedHandler())
	message.RegisterHandler(NewFileNotifiedHandler())
}
