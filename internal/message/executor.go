package message

import (
	"context"
	"fmt"
	"time"

	chainCli "github.com/jackz-jones/blockchain-interactive-service/chaininteractive"
	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/jackz-jones/cross-chain-service/internal/reliability"
	"github.com/jackz-jones/cross-chain-service/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

// baseExecutor 基础执行器，实现 CrossChainExecutor 接口
type baseExecutor struct {
	svcCtx *svc.ServiceContext
	logger logx.Logger
	msgID  string
}

// Execute 执行跨链交易
func (e *baseExecutor) Execute(
	ctx context.Context, targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair,
) (string, error) {
	txId, err := sendCrossChainTx(
		ctx,
		targetChainName,
		targetContractName,
		method,
		kvs,
		e.svcCtx,
	)
	if err != nil {
		e.logger.Errorf("[executor] message %s: send tx failed for method %s: %v", e.msgID, method, err)
		return "", fmt.Errorf("send cross chain tx failed for method %s: %w", method, err)
	}

	e.logger.Infof("[executor] message %s: send tx success for method %s, txId: %s", e.msgID, method, txId)
	return txId, nil
}

// ExecuteWithCallback 执行跨链交易（带回调）
func (e *baseExecutor) ExecuteWithCallback(
	ctx context.Context, targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair,
	callbackMethod string, callbackKvsBuilder func(errMsg string) ([]*chainPb.KeyValuePair, error),
) (string, error) {
	txId, err := e.Execute(ctx, targetChainName, targetContractName, method, kvs)
	if err != nil {
		// 执行失败，发送回调
		if callbackKvsBuilder != nil {
			callbackKvs, err2 := callbackKvsBuilder(err.Error())
			if err2 != nil {
				e.logger.Errorf("[executor] message %s: failed to build callback kvs: %v", e.msgID, err2)
			} else {
				callBackTxId, err3 := sendCrossChainTx(
					ctx,
					targetChainName,
					targetContractName,
					callbackMethod,
					callbackKvs,
					e.svcCtx,
				)
				if err3 != nil {
					e.logger.Errorf("[executor] message %s: failed to send callback tx: %v", e.msgID, err3)
				} else {
					e.logger.Infof("[executor] message %s: callback tx sent, txId: %s", e.msgID, callBackTxId)
				}
			}
		}
		return "", err
	}

	return txId, nil
}

// sendCrossChainTx 发送跨链交易（内部辅助函数）
func sendCrossChainTx(
	ctx context.Context,
	chainConfName, contractConfName, contractMethod string,
	kvs []*chainPb.KeyValuePair,
	svcCtx *svc.ServiceContext,
) (string, error) {
	txResp, err := svcCtx.ChainInteractiveServiceClient.CallContract(ctx, &chainCli.CallContractRequest{
		RequestId:      "cross-chain-service-call",
		ChainName:      chainConfName,
		ContractName:   contractConfName,
		ContractMethod: contractMethod,
		KvPairs:        kvs,
		MethodType:     chainPb.MethodType_Invoke,
		WithSyncResult: svcCtx.Config.SendTxConf.WithSyncResult,
		TxTimeout:      svcCtx.Config.SendTxConf.TxTimeout,
	})
	if err != nil {
		return "", fmt.Errorf("call contract failed: %w", err)
	}

	if txResp.Code != 0 {
		return "", fmt.Errorf("call contract returned error: %s", txResp.Msg)
	}

	return txResp.Data.TxId, nil
}

// CreateReliableExecutor 创建带可靠性的执行器
// 在 message 包内自行实现可靠性逻辑，避免对 event 包的循环依赖
func CreateReliableExecutor(
	svcCtx *svc.ServiceContext,
	logger logx.Logger,
	msgID string,
) CrossChainExecutor {
	rc := svcCtx.Config.ReliabilityConf
	subConf := svcCtx.Config.SubscribeConf

	// 如果未启用可靠性配置，返回基础执行器
	if !rc.EnableIdempotency && !rc.EnableRetry {
		return &baseExecutor{
			svcCtx: svcCtx,
			logger: logger,
			msgID:  msgID,
		}
	}

	// 创建 Redis 适配器
	commonRedis, err := commonEvent.NewRedisClient(
		subConf.ConfType, subConf.RedisAddr,
		subConf.RedisUserName, subConf.RedisPassword, subConf.MasterName,
	)
	if err != nil {
		logger.Errorf("[executor] failed to create redis client: %v", err)
		return &baseExecutor{
			svcCtx: svcCtx,
			logger: logger,
			msgID:  msgID,
		}
	}
	redisAdapter := reliability.NewRedisAdapterFromCommon(commonRedis)

	// 创建可靠性选项
	var idempotChecker reliability.IdempotencyChecker
	var idempotTTL time.Duration
	var retryStrategy *reliability.RetryStrategy
	var taskStore reliability.TaskStore

	if rc.EnableIdempotency {
		idempotChecker = reliability.NewRedisIdempotencyChecker(redisAdapter, "cross_chain:idempotent:")
		idempotTTL = time.Duration(rc.IdempotencyTTL) * time.Second
		if idempotTTL == 0 {
			idempotTTL = 24 * time.Hour
		}
	}

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
		retryStrategy = reliability.NewRetryStrategy(&reliability.RetryConfig{
			MaxRetries: maxRetries,
			BaseDelay:  time.Duration(baseDelay) * time.Millisecond,
			MaxDelay:   time.Duration(maxDelay) * time.Millisecond,
			Multiplier: multiplier,
		})
	}

	taskStore = reliability.NewRedisTaskStore(redisAdapter, "cross_chain:task:", 7*24*time.Hour)

	inner := &baseExecutor{
		svcCtx: svcCtx,
		logger: logger,
		msgID:  msgID,
	}

	return &reliableExecutor{
		inner:       inner,
		idempotency: idempotChecker,
		taskStore:   taskStore,
		retry:       retryStrategy,
		logger:      logger,
		eventName:   msgID,
		idempotTTL:  idempotTTL,
	}
}

// reliableExecutor 可靠的跨链交易执行器
// 在 message 包中实现，避免对 event 包的循环依赖
type reliableExecutor struct {
	inner       CrossChainExecutor
	idempotency reliability.IdempotencyChecker
	taskStore   reliability.TaskStore
	retry       *reliability.RetryStrategy
	logger      logx.Logger
	eventName   string
	idempotTTL  time.Duration
}

// Execute 执行跨链交易（带可靠性保障）
func (e *reliableExecutor) Execute(
	ctx context.Context, targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair,
) (string, error) {

	// 1. 幂等性检查
	eventKey := targetChainName + ":" + targetContractName + ":" + method
	if e.idempotency != nil {
		dup, err := e.idempotency.IsDuplicate(ctx, eventKey)
		if err != nil {
			e.logger.Errorf("[%s] idempotency check error: %v", e.eventName, err)
		} else if dup {
			e.logger.Infof("[%s] duplicate event detected, skipping: %s", e.eventName, eventKey)
			return "", nil
		}
	}

	// 2. 执行（带重试）
	var txId string
	var execErr error

	if e.retry != nil {
		execErr = e.retry.ExecuteWithRetryImmediate(func() error {
			var err error
			txId, err = e.inner.Execute(ctx, targetChainName, targetContractName, method, kvs)
			return err
		})
	} else {
		txId, execErr = e.inner.Execute(ctx, targetChainName, targetContractName, method, kvs)
	}

	// 3. 标记已处理
	if execErr == nil && e.idempotency != nil {
		if err := e.idempotency.MarkProcessed(ctx, eventKey, e.idempotTTL); err != nil {
			e.logger.Errorf("[%s] failed to mark event as processed: %v", e.eventName, err)
		}
	}

	// 4. 记录任务状态
	if e.taskStore != nil {
		task := &reliability.CrossChainTask{
			TaskID:       eventKey,
			EventKey:     eventKey,
			TargetChain:  targetChainName,
			ContractName: targetContractName,
			Method:       method,
			TxID:         txId,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		if execErr != nil {
			task.State = reliability.TaskStateFailed
			task.ErrorMsg = execErr.Error()
		} else {
			task.State = reliability.TaskStateConfirmed
		}
		if err := e.taskStore.Save(ctx, task); err != nil {
			e.logger.Errorf("[%s] failed to save task: %v", e.eventName, err)
		}
	}

	return txId, execErr
}

// ExecuteWithCallback 执行跨链交易（带回调和可靠性保障）
func (e *reliableExecutor) ExecuteWithCallback(
	ctx context.Context, targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair,
	callbackMethod string, callbackKvsBuilder func(errMsg string) ([]*chainPb.KeyValuePair, error),
) (string, error) {

	// 1. 幂等性检查
	eventKey := targetChainName + ":" + targetContractName + ":" + method
	if e.idempotency != nil {
		dup, err := e.idempotency.IsDuplicate(ctx, eventKey)
		if err != nil {
			e.logger.Errorf("[%s] idempotency check error: %v", e.eventName, err)
		} else if dup {
			e.logger.Infof("[%s] duplicate event detected, skipping: %s", e.eventName, eventKey)
			return "", nil
		}
	}

	// 2. 执行（带重试和回调）
	var txId string
	var execErr error

	if e.retry != nil {
		execErr = e.retry.ExecuteWithRetryImmediate(func() error {
			var err error
			txId, err = e.inner.ExecuteWithCallback(
				ctx, targetChainName, targetContractName,
				method, kvs, callbackMethod, callbackKvsBuilder,
			)
			return err
		})
	} else {
		txId, execErr = e.inner.ExecuteWithCallback(
			ctx, targetChainName, targetContractName,
			method, kvs, callbackMethod, callbackKvsBuilder,
		)
	}

	// 3. 标记已处理
	if execErr == nil && e.idempotency != nil {
		if err := e.idempotency.MarkProcessed(ctx, eventKey, e.idempotTTL); err != nil {
			e.logger.Errorf("[%s] failed to mark event as processed: %v", e.eventName, err)
		}
	}

	return txId, execErr
}

// CrossChainExecutor 跨链交易执行器接口
type CrossChainExecutor interface {
	Execute(ctx context.Context,
		targetChainName, targetContractName, method string,
		kvs []*chainPb.KeyValuePair) (string, error)
	ExecuteWithCallback(ctx context.Context,
		targetChainName, targetContractName, method string,
		kvs []*chainPb.KeyValuePair,
		callbackMethod string,
		callbackKvsBuilder func(errMsg string) ([]*chainPb.KeyValuePair, error),
	) (string, error)
}
