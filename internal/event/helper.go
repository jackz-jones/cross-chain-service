package event

import (
	"context"
	"fmt"
	"time"

	"github.com/jackz-jones/cross-chain-service/internal/code"
	"github.com/jackz-jones/cross-chain-service/internal/plugins/nft"
	"github.com/jackz-jones/cross-chain-service/internal/plugins/notification"

	"github.com/jackz-jones/blockchain-interactive-service/chaininteractive"
	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
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

// ==========================================
// 业务 KVS 构建函数（代理到插件模块）
// 保留这些函数以兼容旧的事件处理器代码
// ==========================================

// CreateNotifyEnterpriseInfoKvs 构造调用合约方法 NotifyEnterpriseInfo 参数 kvs
// 代理到 notification 插件模块
func CreateNotifyEnterpriseInfoKvs(id, originHash, address, did string,
	needCrossChain bool) ([]*chainPb.KeyValuePair, error) {
	return notification.CreateNotifyEnterpriseInfoKvs(id, originHash, address, did, needCrossChain)
}

// CreateNotifyFileInfoKvs 构造调用合约方法 NotifyFileInfo 参数 kvs
// 代理到 notification 插件模块
func CreateNotifyFileInfoKvs(id, originHash string, msgType int, needCrossChain bool) ([]*chainPb.KeyValuePair, error) {
	return notification.CreateNotifyFileInfoKvs(id, originHash, msgType, needCrossChain)
}

// CreateCrossChainMintKvs 构造调用合约方法 CrossChainMint 参数 kvs
// 代理到 nft 插件模块
func CreateCrossChainMintKvs(id, owner, holder, originHash, data string) ([]*chainPb.KeyValuePair, error) {
	return nft.CreateCrossChainMintKvs(id, owner, holder, originHash, data)
}

// CreateUpdateCrossChainStatusKvs 构造调用合约方法 UpdateCrossChainStatus 参数 kvs
// 代理到 nft 插件模块
func CreateUpdateCrossChainStatusKvs(tokenId string, state int) ([]*chainPb.KeyValuePair, error) {
	return nft.CreateUpdateCrossChainStatusKvs(tokenId, state)
}

// CreateCallbackKvs 构造调用合约方法 Callback 参数 kvs
// 代理到 notification 插件模块
func CreateCallbackKvs(id, originHash, msg string, errCode, msgType int) ([]*chainPb.KeyValuePair, error) {
	return notification.CreateCallbackKvs(id, originHash, msg, errCode, msgType)
}

// ==========================================
// 时间相关辅助函数
// ==========================================

// Now 返回当前时间（便于测试中 mock）
var Now = time.Now
