// Package event 合约事件处理
package event

import (
	"fmt"
	"time"

	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	"github.com/jackz-jones/cross-chain-service/internal/code"
	"github.com/jackz-jones/cross-chain-service/internal/reliability"
	"github.com/jackz-jones/cross-chain-service/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"

	chainCli "github.com/jackz-jones/blockchain-interactive-service/chaininteractive"
)

// CrossChainExecutor 跨链交易执行器接口
type CrossChainExecutor interface {
	// Execute 执行跨链交易
	// targetChainName: 目标链名称
	// targetContractName: 目标合约名称
	// method: 调用方法
	// kvs: 参数列表
	// 返回交易 ID 和错误
	Execute(targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair) (string, error)

	// ExecuteWithCallback 执行跨链交易，失败时发送回调
	// callbackMethod: 回调方法名
	// callbackKvsBuilder: 构造回调参数的函数（接收错误信息）
	ExecuteWithCallback(targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair,
		callbackMethod string, callbackKvsBuilder func(errMsg string) ([]*chainPb.KeyValuePair, error)) (string, error)
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

// newCrossChainExecutor 根据配置创建跨链交易执行器
// 如果启用了可靠性配置（幂等性或重试），则创建 ReliableCrossChainExecutor
// 否则创建默认的 DefaultCrossChainExecutor
func newCrossChainExecutor(
	logger logx.Logger,
	svcCtx *svc.ServiceContext,
	crossTargetChainConf *chainCli.ChainAndContractName,
	eventName string,
) CrossChainExecutor {
	rc := svcCtx.Config.ReliabilityConf
	subConf := svcCtx.Config.SubscribeConf

	// 如果启用了幂等性检查或重试机制，使用可靠执行器
	if rc.EnableIdempotency || rc.EnableRetry {
		opts := []ReliableExecutorOption{}

		// 配置幂等性检查
		if rc.EnableIdempotency {
			redisClient := reliability.NewGoRedisClient(
				subConf.ConfType, subConf.RedisAddr,
				subConf.RedisUserName, subConf.RedisPassword, subConf.MasterName,
			)
			checker := reliability.NewRedisIdempotencyChecker(
				redisClient,
				"cross_chain:idempotent:",
			)
			idempotTTL := time.Duration(rc.IdempotencyTTL) * time.Second
			if idempotTTL == 0 {
				idempotTTL = 24 * time.Hour
			}
			opts = append(opts, WithIdempotency(checker, idempotTTL))
		}

		// 配置重试策略
		if rc.EnableRetry {
			maxRetries := rc.MaxRetries
			if maxRetries == 0 {
				maxRetries = 3
			}
			baseDelay := rc.RetryBaseDelay
			if baseDelay == 0 {
				baseDelay = 1000
			}
			maxDelay := rc.RetryMaxDelay
			if maxDelay == 0 {
				maxDelay = 30000
			}
			multiplier := rc.RetryMultiplier
			if multiplier == 0 {
				multiplier = 2.0
			}
			retryStrategy := reliability.NewRetryStrategy(&reliability.RetryConfig{
				MaxRetries: maxRetries,
				BaseDelay:  time.Duration(baseDelay) * time.Millisecond,
				MaxDelay:   time.Duration(maxDelay) * time.Millisecond,
				Multiplier: multiplier,
			})
			opts = append(opts, WithRetry(retryStrategy))
		}

		// 配置任务持久化
		redisHashClient := reliability.NewGoRedisHashClient(
			subConf.ConfType, subConf.RedisAddr,
			subConf.RedisUserName, subConf.RedisPassword, subConf.MasterName,
		)
		taskStore := reliability.NewRedisTaskStore(redisHashClient, "cross_chain:task:", 7*24*time.Hour)
		opts = append(opts, WithTaskStore(taskStore))

		defaultExecutor := NewDefaultCrossChainExecutor(logger, svcCtx, crossTargetChainConf, eventName)
		return NewReliableCrossChainExecutor(defaultExecutor, logger, eventName, opts...)
	}

	// 默认使用基础执行器
	return NewDefaultCrossChainExecutor(logger, svcCtx, crossTargetChainConf, eventName)
}

// Execute 执行跨链交易
func (e *DefaultCrossChainExecutor) Execute(targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair) (string, error) {
	txId, err := SendCrossChainTx(
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
	targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair,
	callbackMethod string, callbackKvsBuilder func(errMsg string) ([]*chainPb.KeyValuePair, error),
) (string, error) {
	txId, err := e.Execute(targetChainName, targetContractName, method, kvs)
	if err != nil {
		// 发送失败，尝试回调
		if callbackKvsBuilder != nil {
			callbackKvs, err2 := callbackKvsBuilder(err.Error())
			if err2 != nil {
				e.logger.Errorf("[%s] %s: %v", e.eventName, code.ErrMsgCreateCallbackKvs, err2)
			} else {
				callBackTxId, err3 := SendCrossChainTx(
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
