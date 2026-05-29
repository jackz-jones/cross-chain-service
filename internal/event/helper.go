package event

import (
	"context"
	"fmt"

	"github.com/jackz-jones/cross-chain-service/internal/code"

	"github.com/jackz-jones/blockchain-interactive-service/chaininteractive"
	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
)

// SendCrossChainTx 发送跨链交易
func SendCrossChainTx(
	ctx context.Context,
	chainConfName, contractConfName, contractMethod string,
	kvs []*chainPb.KeyValuePair,
	methodType chainPb.MethodType, withSyncResult bool, txTimeout int64,
	chainInteractiveServiceClient chaininteractive.ChainInteractive,
) (string, error) {

	// 发送跨链交易
	txResp, err := chainInteractiveServiceClient.CallContract(ctx, &chaininteractive.CallContractRequest{
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
