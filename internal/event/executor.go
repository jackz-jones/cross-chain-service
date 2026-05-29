// Package event 合约事件处理
package event

import (
	"context"
	"fmt"

	chainCli "github.com/jackz-jones/blockchain-interactive-service/chaininteractive"
	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	"github.com/jackz-jones/cross-chain-service/internal/code"
	"github.com/jackz-jones/cross-chain-service/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

// CrossChainExecutor 跨链交易执行器接口
type CrossChainExecutor interface {
	// Execute 执行跨链交易
	// ctx: 上下文，用于超时控制和优雅退出
	// targetChainName: 目标链名称
	// targetContractName: 目标合约名称
	// method: 调用方法
	// kvs: 参数列表
	// 返回交易 ID 和错误
	Execute(ctx context.Context,
		targetChainName, targetContractName, method string,
		kvs []*chainPb.KeyValuePair) (string, error)

	// ExecuteWithCallback 执行跨链交易，失败时发送回调
	// callbackMethod: 回调方法名
	// callbackKvsBuilder: 构造回调参数的函数（接收错误信息）
	ExecuteWithCallback(ctx context.Context,
		targetChainName, targetContractName, method string,
		kvs []*chainPb.KeyValuePair,
		callbackMethod string,
		callbackKvsBuilder func(errMsg string) ([]*chainPb.KeyValuePair, error),
	) (string, error)
}

// DefaultCrossChainExecutor 默认跨链交易执行器实现
type DefaultCrossChainExecutor struct {
	svcCtx               *svc.ServiceContext
	logger               logx.Logger
	crossTargetChainConf *chainCli.ChainAndContractName
	eventName            string
}

// NewDefaultCrossChainExecutor 创建默认跨链交易执行器
func NewDefaultCrossChainExecutor(
	logger logx.Logger,
	svcCtx *svc.ServiceContext,
	crossTargetChainConf *chainCli.ChainAndContractName,
	eventName string,
) *DefaultCrossChainExecutor {
	return &DefaultCrossChainExecutor{
		svcCtx:               svcCtx,
		logger:               logger,
		crossTargetChainConf: crossTargetChainConf,
		eventName:            eventName,
	}
}

// Execute 执行跨链交易
func (e *DefaultCrossChainExecutor) Execute(
	ctx context.Context, targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair,
) (string, error) {
	txId, err := SendCrossChainTx(
		ctx,
		targetChainName,
		targetContractName,
		method,
		kvs,
		chainPb.MethodType_Invoke,
		e.svcCtx.Config.SendTxConf.WithSyncResult,
		e.svcCtx.Config.SendTxConf.TxTimeout,
		e.svcCtx.ChainInteractiveServiceClient,
	)
	if err != nil {
		e.logger.Errorf("[%s] %s %s: %v", e.eventName, code.ErrMsgSendCrossChainTx, method, err)
		return "", fmt.Errorf("%s %s: %v", code.ErrMsgSendCrossChainTx, method, err)
	}

	e.logger.Infof("[%s] %s %s: %s", e.eventName, code.MsgSuccessToSendCrossChainTx, method, txId)
	return txId, nil
}

// ExecuteWithCallback 执行跨链交易，失败时发送回调
func (e *DefaultCrossChainExecutor) ExecuteWithCallback(
	ctx context.Context, targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair,
	callbackMethod string, callbackKvsBuilder func(errMsg string) ([]*chainPb.KeyValuePair, error),
) (string, error) {
	txId, err := e.Execute(ctx, targetChainName, targetContractName, method, kvs)
	if err != nil {
		// 发送失败，尝试回调
		if callbackKvsBuilder != nil {
			callbackKvs, err2 := callbackKvsBuilder(err.Error())
			if err2 != nil {
				e.logger.Errorf("[%s] %s: %v", e.eventName, code.ErrMsgCreateCallbackKvs, err2)
			} else {
				callBackTxId, err3 := SendCrossChainTx(
					ctx,
					targetChainName,
					targetContractName,
					callbackMethod,
					callbackKvs,
					chainPb.MethodType_Invoke,
					e.svcCtx.Config.SendTxConf.WithSyncResult,
					e.svcCtx.Config.SendTxConf.TxTimeout,
					e.svcCtx.ChainInteractiveServiceClient,
				)
				if err3 != nil {
					e.logger.Errorf("[%s] %s Callback: %v", e.eventName, code.ErrMsgSendCrossChainTx, err3)
				} else {
					e.logger.Infof("[%s] %s Callback: %s", e.eventName, code.MsgSuccessToSendCrossChainTx, callBackTxId)
				}
			}
		}
		return "", err
	}

	return txId, nil
}
