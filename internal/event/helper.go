package event

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackz-jones/cross-chain-service/internal/code"

	"github.com/jackz-jones/blockchain-interactive-service/chaininteractive"
	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	nftConst "github.com/jackz-jones/nft-contract-go/const"
	nftTypes "github.com/jackz-jones/nft-contract-go/types"
	notificationConst "github.com/jackz-jones/notification-contract-go/const"
	notificationTypes "github.com/jackz-jones/notification-contract-go/types"
	notificationUtil "github.com/jackz-jones/notification-contract-go/util"
)

// CheckWhitelist 检查地址是否都在白名单
func CheckWhitelist(addrList []string) error {

	// TODO：收集白名单
	whiteListMap := make(map[string]bool, 0)

	// 检查每个地址是否都在白名单
	for _, addr := range addrList {
		if !whiteListMap[addr] {
			return fmt.Errorf("%s %s", addr, code.ErrMsgNotInWhiteList)
		}
	}

	// 检查白名单成功
	return nil
}

// NotifyFile 通知文件
func NotifyFile(id, address string, messageType int) error {

	// TODO：发送文件通知

	// 返回成功
	return nil
}

// SendCrossChainTx 发送跨链交易
func SendCrossChainTx(chainConfName, contractConfName, contractMethod string, kvs []*chainPb.KeyValuePair,
	methodType chainPb.MethodType, withSyncResult bool, txTimeout int64,
	chainInteractiveServiceClient chaininteractive.ChainInteractive) (string, error) {

	// 发送跨链交易
	txResp, err := chainInteractiveServiceClient.CallContract(context.Background(), &chaininteractive.CallContractRequest{
		RequestId: "cross-chain-service-call-contract",

		// todo: 目标链可以由跨链服务
		ChainName: chainConfName,

		// todo: 目标合约、方法、参数应该由业务端传过来才通用化，这里就必须传实际的合约名称或者以太坊的 address 了
		ContractName:   contractConfName,
		ContractMethod: contractMethod,
		KvPairs:        kvs,
		MethodType:     methodType,
		WithSyncResult: withSyncResult,
		TxTimeout:      txTimeout,
	})
	if err != nil {
		return "", fmt.Errorf("%s: %v", code.ErrMsgSendCallContract, err)
	}

	// 检查 grpc 返回码
	if txResp.Code != int32(code.Success) {
		return "", fmt.Errorf("%s: %v", code.ErrMsgExecuteCallContract, txResp.Msg)
	}

	// 返回成功
	return txResp.Data.TxId, nil
}

// CreateNotifyEnterpriseInfoKvs 构造调用合约方法 NotifyEnterpriseInfo 参数 kvs，
// 由 chain-interactive-service 服务本身去翻译成以太坊的 input data
func CreateNotifyEnterpriseInfoKvs(id, originHash, address, did string,
	needCrossChain bool) ([]*chainPb.KeyValuePair, error) {

	// 企业身份信息
	ei := notificationTypes.EnterpriseInfo{
		ID:         id,
		OriginHash: originHash,
		CreatedAt:  time.Now(),
		Address:    address,
		Did:        did,
	}
	eiBytes, err := json.Marshal(ei)
	if err != nil {
		return nil, fmt.Errorf("%s EnterpriseInfo: %v", code.ErrMsgJsonMarshal, err)
	}

	// 组装交易 kvs 参数
	return []*chainPb.KeyValuePair{
		{
			Key:   notificationConst.ParamEnterpriseInfo,
			Value: eiBytes,
		},
		{
			Key:   notificationConst.ParamNeedCrossChain,
			Value: notificationUtil.BoolToBytes(needCrossChain),
		},
	}, nil
}

// CreateNotifyFileInfoKvs 构造调用合约方法 NotifyFileInfo 参数 kvs，由 chain-interactive-service 服务本身去翻译成以太坊的 input data
func CreateNotifyFileInfoKvs(id, originHash string, msgType int, needCrossChain bool) ([]*chainPb.KeyValuePair, error) {

	// 文件信息
	fi := notificationTypes.FileInfo{
		ID:         id,
		OriginHash: originHash,
		CreatedAt:  time.Now(),
		MsgType:    notificationTypes.MessageType(msgType),
	}
	fiBytes, err := json.Marshal(fi)
	if err != nil {
		return nil, fmt.Errorf("%s FileInfo: %v", code.ErrMsgJsonMarshal, err)
	}

	// 组装交易 kvs 参数
	return []*chainPb.KeyValuePair{
		{
			Key:   notificationConst.ParamFileInfo,
			Value: fiBytes,
		},
		{
			Key:   notificationConst.ParamNeedCrossChain,
			Value: notificationUtil.BoolToBytes(needCrossChain),
		},
	}, nil
}

// CreateCrossChainMintKvs 构造调用合约方法 CrossChainMint 参数 kvs，由 chain-interactive-service 服务本身去翻译成以太坊的 input data
func CreateCrossChainMintKvs(id, owner, holder, originHash, data string) ([]*chainPb.KeyValuePair, error) {

	// nft信息
	ni := nftTypes.NFTInfo{
		ID:         id,
		Owner:      owner,
		Holder:     holder,
		OriginHash: originHash,
		CreatedAt:  time.Now(),
		Data:       data,
	}
	niBytes, err := json.Marshal(ni)
	if err != nil {
		return nil, fmt.Errorf("%s NFTInfo: %v", code.ErrMsgJsonMarshal, err)
	}

	// 组装交易 kvs 参数
	return []*chainPb.KeyValuePair{
		{
			Key:   nftConst.ParamNFTInfo,
			Value: niBytes,
		},
	}, nil
}

// CreateUpdateCrossChainStatusKvs 构造调用合约方法 UpdateCrossChainStatus 参数 kvs，
// 由 chain-interactive-service 服务本身去翻译成以太坊的 input data
func CreateUpdateCrossChainStatusKvs(tokenId string, state int) ([]*chainPb.KeyValuePair, error) {

	// 组装交易 kvs 参数
	return []*chainPb.KeyValuePair{
		{
			Key:   nftConst.ParamTokenId,
			Value: []byte(tokenId),
		},
		{
			Key:   nftConst.ParamState,
			Value: []byte(fmt.Sprintf("%d", state)),
		},
	}, nil
}

// CreateCallbackKvs 构造调用合约方法 Callback 参数 kvs，由 chain-interactive-service 服务本身去翻译成以太坊的 input data
func CreateCallbackKvs(id, originHash, msg string, errCode, msgType int) ([]*chainPb.KeyValuePair, error) {

	// callback信息
	cc := notificationTypes.CallBackInfo{
		ID:         id,
		OriginHash: originHash,
		Code:       errCode,
		Msg:        msg,
		MsgType:    notificationTypes.MessageType(msgType),
	}
	ccBytes, err := json.Marshal(cc)
	if err != nil {
		return nil, fmt.Errorf("%s CallBackInfo: %v", code.ErrMsgJsonMarshal, err)
	}

	// 组装交易 kvs 参数
	return []*chainPb.KeyValuePair{
		{
			Key:   notificationConst.ParamCallbackInfo,
			Value: ccBytes,
		},
	}, nil
}
