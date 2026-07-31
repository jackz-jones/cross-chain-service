package message

import (
	"context"
	"fmt"
	"time"

	chainCli "github.com/jackz-jones/blockchain-interactive-service/chaininteractive"
	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
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
	chainClient := svcCtx.GetChainInteractiveClient()
	if chainClient == nil {
		return "", svc.ErrChainInteractiveNotReady
	}
	txResp, err := chainClient.CallContract(ctx, &chainCli.CallContractRequest{
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
// 在 message 包内自行实现可靠性逻辑，避免对 event 包的循环依赖。
// msg 为可选参数：如果传入非 nil，会在执行过程中同步更新其
// Status/TxID/ErrorMessage/RetryCount 字段，并将该消息本身持久化到 TaskStore。
func CreateReliableExecutor(
	svcCtx *svc.ServiceContext,
	logger logx.Logger,
	msg *CrossChainMessage,
) CrossChainExecutor {
	rc := svcCtx.Config.ReliabilityConf
	msgID := ""
	if msg != nil {
		msgID = msg.MessageID
	}

	// 如果未启用可靠性配置，返回基础执行器
	if !rc.EnableIdempotency && !rc.EnableRetry {
		return &baseExecutor{
			svcCtx: svcCtx,
			logger: logger,
			msgID:  msgID,
		}
	}

	// 复用 ServiceContext 中的共享 Redis 客户端，避免每条消息都新建连接
	commonRedis := svcCtx.SharedRedisClient
	if commonRedis == nil {
		logger.Errorf("[executor] shared redis client is nil, fallback to base executor")
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
		msg:         msg,
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
	// msg 为可选的关联消息；非 nil 时执行器会同步更新其状态字段
	msg *CrossChainMessage
}

// persistTask 将当前执行结果持久化到 TaskStore；如果构造时传入了 msg，
// 会先把 Status/TxID/ErrorMessage/RetryCount 回写到消息本身，保持消息与任务状态一致。
func (e *reliableExecutor) persistTask(
	ctx context.Context,
	eventKey, targetChainName, targetContractName, method, txID string,
	execErr error,
) {
	if e.taskStore == nil {
		return
	}

	var state reliability.TaskState
	var errMsg string
	if execErr != nil {
		state = reliability.TaskStateFailed
		errMsg = execErr.Error()
	} else {
		state = reliability.TaskStateConfirmed
	}

	task := &reliability.CrossChainTask{
		TaskID:       e.eventName,
		EventKey:     eventKey,
		TargetChain:  targetChainName,
		ContractName: targetContractName,
		Method:       method,
		State:        state,
		TxID:         txID,
		ErrorMsg:     errMsg,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if e.msg != nil {
		// 同步更新消息状态，让消息本身与任务视图保持一致
		e.msg.Status = MessageStatus(state)
		if txID != "" {
			e.msg.TxID = txID
		}
		e.msg.ErrorMessage = errMsg
		task.SourceChain = e.msg.SourceChain
		task.RetryCount = e.msg.RetryCount
		if !e.msg.CreatedAt.IsZero() {
			task.CreatedAt = e.msg.CreatedAt
		}
	}

	if err := e.taskStore.Save(ctx, task); err != nil {
		e.logger.Errorf("[%s] failed to save task: %v", e.eventName, err)
	}
}

// Execute 执行跨链交易（带可靠性保障）
func (e *reliableExecutor) Execute(
	ctx context.Context, targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair,
) (string, error) {

	// 1. 幂等性检查：eventKey 必须包含消息唯一标识（MessageID），
	//    否则同类型不同消息会被误判为重复而被吞掉。
	eventKey := e.eventName + ":" + targetChainName + ":" + targetContractName + ":" + method
	if e.idempotency != nil {
		dup, err := e.idempotency.IsDuplicate(ctx, eventKey)
		if err != nil {
			e.logger.Errorf("[%s] idempotency check error (treat as not duplicate): %v", e.eventName, err)
		} else if dup {
			e.logger.Infof("[%s] duplicate event detected, skipping: %s", e.eventName, eventKey)
			return "", nil
		}
	}

	// 2. 执行（带重试 + ctx 中止）
	var txId string
	var execErr error

	if e.retry != nil {
		execErr = e.retry.ExecuteWithRetryCtx(ctx, func() error {
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

	// 4. 记录任务状态：TaskID 使用 MessageID 保证唯一，EventKey 用作幂等标识
	e.persistTask(ctx, eventKey, targetChainName, targetContractName, method, txId, execErr)

	return txId, execErr
}

// ExecuteWithCallback 执行跨链交易（带回调和可靠性保障）
func (e *reliableExecutor) ExecuteWithCallback(
	ctx context.Context, targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair,
	callbackMethod string, callbackKvsBuilder func(errMsg string) ([]*chainPb.KeyValuePair, error),
) (string, error) {

	// 1. 幂等性检查：eventKey 必须包含消息唯一标识（MessageID）
	eventKey := e.eventName + ":" + targetChainName + ":" + targetContractName + ":" + method
	if e.idempotency != nil {
		dup, err := e.idempotency.IsDuplicate(ctx, eventKey)
		if err != nil {
			e.logger.Errorf("[%s] idempotency check error (treat as not duplicate): %v", e.eventName, err)
		} else if dup {
			e.logger.Infof("[%s] duplicate event detected, skipping: %s", e.eventName, eventKey)
			return "", nil
		}
	}

	// 2. 执行（带重试 + ctx 中止 + 回调）
	var txId string
	var execErr error

	if e.retry != nil {
		execErr = e.retry.ExecuteWithRetryCtx(ctx, func() error {
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

	// 4. 记录任务状态：与 Execute 行为对齐，无论成功失败都落盘
	e.persistTask(ctx, eventKey, targetChainName, targetContractName, method, txId, execErr)

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
