package message

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	chainCli "github.com/jackz-jones/blockchain-interactive-service/chaininteractive"
	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/jackz-jones/cross-chain-service/internal/reliability"
	"github.com/jackz-jones/cross-chain-service/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

// GenericExecutor 通用跨链消息执行器
// 接受 CrossChainMessage，通过路由查找目标，调用适配器发送交易
// 支持与现有可靠性机制（ReliableCrossChainExecutor）对接
type GenericExecutor struct {
	svcCtx *svc.ServiceContext
	router *MessageRouter
	logger logx.Logger
}

// NewGenericExecutor 创建通用执行器
func NewGenericExecutor(svcCtx *svc.ServiceContext, router *MessageRouter, logger logx.Logger) *GenericExecutor {
	return &GenericExecutor{
		svcCtx: svcCtx,
		router: router,
		logger: logger,
	}
}

// Execute 执行跨链消息
// 根据消息路由找到目标链和合约，发送跨链交易
func (e *GenericExecutor) Execute(msg *CrossChainMessage) (string, error) {
	// 1. 路由查找
	targets, err := e.router.Route(msg)
	if err != nil {
		return "", fmt.Errorf("route failed for message %s: %w", msg.MessageID, err)
	}
	if len(targets) == 0 {
		e.logger.Infof("[generic-executor] message %s discarded (no route found)", msg.MessageID)
		return "", nil
	}

	// 2. 目前仅支持单目标路由（第一个匹配）
	target := targets[0]

	// 3. 构建 KeyValuePair 参数
	kvs, err := e.buildKvsFromPayload(msg.Payload)
	if err != nil {
		return "", fmt.Errorf("failed to build kvs from payload for message %s: %w", msg.MessageID, err)
	}

	// 4. 创建执行器并执行
	executor := e.createExecutor(msg, target)
	txId, err := executor.Execute(
		target.TargetChain,
		target.TargetContract,
		target.Method,
		kvs,
	)
	if err != nil {
		// 执行失败，尝试回调
		e.handleCallbackOnFailure(msg, target, err)
		return "", err
	}

	e.logger.Infof("[generic-executor] message %s executed successfully, txId: %s", msg.MessageID, txId)
	return txId, nil
}

// ExecuteWithCallback 执行跨链消息（带回调）
func (e *GenericExecutor) ExecuteWithCallback(msg *CrossChainMessage) (string, error) {
	// 1. 路由查找
	targets, err := e.router.Route(msg)
	if err != nil {
		return "", fmt.Errorf("route failed for message %s: %w", msg.MessageID, err)
	}
	if len(targets) == 0 {
		e.logger.Infof("[generic-executor] message %s discarded (no route found)", msg.MessageID)
		return "", nil
	}

	target := targets[0]

	// 2. 构建 KeyValuePair 参数
	kvs, err := e.buildKvsFromPayload(msg.Payload)
	if err != nil {
		return "", fmt.Errorf("failed to build kvs from payload for message %s: %w", msg.MessageID, err)
	}

	// 3. 创建执行器
	executor := e.createExecutor(msg, target)

	// 4. 执行（带回调）
	if msg.HasCallback() {
		callbackKvsBuilder := func(errMsg string) ([]*chainPb.KeyValuePair, error) {
			return e.buildCallbackKvs(msg, errMsg)
		}
		resultTxId, execErr := executor.ExecuteWithCallback(
			target.TargetChain,
			target.TargetContract,
			target.Method,
			kvs,
			msg.CallbackMethod,
			callbackKvsBuilder,
		)
		if execErr != nil {
			return "", execErr
		}
		return resultTxId, nil
	}

	// 无回调，直接执行
	resultTxId, execErr := executor.Execute(
		target.TargetChain,
		target.TargetContract,
		target.Method,
		kvs,
	)
	if execErr != nil {
		return "", execErr
	}

	return resultTxId, nil
}

// createExecutor 根据配置创建跨链执行器
// 如果启用了可靠性配置，则创建 ReliableCrossChainExecutor；否则创建 DefaultCrossChainExecutor
func (e *GenericExecutor) createExecutor(msg *CrossChainMessage, target RouteTarget) CrossChainExecutor {
	// 注意：这里暂时返回基于通用消息的执行器
	// 后续可以通过 svcCtx 获取可靠性配置，创建对应的 ReliableCrossChainExecutor
	return &genericInnerExecutor{
		svcCtx: e.svcCtx,
		logger: e.logger,
		msgID:  msg.MessageID,
	}
}

// handleCallbackOnFailure 执行失败时处理回调
func (e *GenericExecutor) handleCallbackOnFailure(msg *CrossChainMessage, target RouteTarget, execErr error) {
	if !msg.HasCallback() {
		return
	}

	callbackKvs, err := e.buildCallbackKvs(msg, execErr.Error())
	if err != nil {
		e.logger.Errorf("[generic-executor] failed to build callback kvs for message %s: %v", msg.MessageID, err)
		return
	}

	// 发送回调交易
	txId, err := sendCrossChainTx(
		target.TargetChain,
		target.TargetContract,
		msg.CallbackMethod,
		callbackKvs,
		e.svcCtx,
	)
	if err != nil {
		e.logger.Errorf("[generic-executor] failed to send callback for message %s: %v", msg.MessageID, err)
	} else {
		e.logger.Infof("[generic-executor] callback sent for message %s, txId: %s", msg.MessageID, txId)
	}
}

// buildKvsFromPayload 从 Payload 字节构建 KeyValuePair 列表
// Payload 可以是 JSON 编码的 []*chainPb.KeyValuePair 或 map[string]interface{}
func (e *GenericExecutor) buildKvsFromPayload(payload []byte) ([]*chainPb.KeyValuePair, error) {
	if len(payload) == 0 {
		return nil, nil
	}

	// 尝试解析为 KeyValuePair 列表
	var kvs []*chainPb.KeyValuePair
	if err := json.Unmarshal(payload, &kvs); err == nil {
		return kvs, nil
	}

	// 尝试解析为 map 格式
	var kvMap map[string]json.RawMessage
	if err := json.Unmarshal(payload, &kvMap); err == nil {
		for k, v := range kvMap {
			kvs = append(kvs, &chainPb.KeyValuePair{
				Key:   k,
				Value: v,
			})
		}
		return kvs, nil
	}

	// 兜底：整个 payload 作为一个 KeyValuePair
	return []*chainPb.KeyValuePair{
		{
			Key:   "payload",
			Value: payload,
		},
	}, nil
}

// buildCallbackKvs 构建回调参数
func (e *GenericExecutor) buildCallbackKvs(msg *CrossChainMessage, errMsg string) ([]*chainPb.KeyValuePair, error) {
	if len(msg.CallbackPayload) > 0 {
		return e.buildKvsFromPayload(msg.CallbackPayload)
	}

	// 默认回调参数：包含错误信息
	type callbackData struct {
		MessageID string `json:"messageId"`
		Success   bool   `json:"success"`
		ErrorMsg  string `json:"errorMsg,omitempty"`
	}
	data := callbackData{
		MessageID: msg.MessageID,
		Success:   errMsg == "",
		ErrorMsg:  errMsg,
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal callback data: %w", err)
	}

	return []*chainPb.KeyValuePair{
		{
			Key:   "callbackInfo",
			Value: dataBytes,
		},
	}, nil
}

// genericInnerExecutor 通用内部执行器，实现 CrossChainExecutor 接口
type genericInnerExecutor struct {
	svcCtx *svc.ServiceContext
	logger logx.Logger
	msgID  string
}

// Execute 执行跨链交易
func (e *genericInnerExecutor) Execute(
	targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair,
) (string, error) {
	txId, err := sendCrossChainTx(
		targetChainName,
		targetContractName,
		method,
		kvs,
		e.svcCtx,
	)
	if err != nil {
		e.logger.Errorf("[generic-executor] message %s: send tx failed for method %s: %v", e.msgID, method, err)
		return "", fmt.Errorf("send cross chain tx failed for method %s: %w", method, err)
	}

	e.logger.Infof("[generic-executor] message %s: send tx success for method %s, txId: %s", e.msgID, method, txId)
	return txId, nil
}

// ExecuteWithCallback 执行跨链交易（带回调）
func (e *genericInnerExecutor) ExecuteWithCallback(
	targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair,
	callbackMethod string, callbackKvsBuilder func(errMsg string) ([]*chainPb.KeyValuePair, error),
) (string, error) {
	txId, err := e.Execute(targetChainName, targetContractName, method, kvs)
	if err != nil {
		// 执行失败，发送回调
		if callbackKvsBuilder != nil {
			callbackKvs, err2 := callbackKvsBuilder(err.Error())
			if err2 != nil {
				e.logger.Errorf("[generic-executor] message %s: failed to build callback kvs: %v", e.msgID, err2)
			} else {
				callBackTxId, err3 := sendCrossChainTx(
					targetChainName,
					targetContractName,
					callbackMethod,
					callbackKvs,
					e.svcCtx,
				)
				if err3 != nil {
					e.logger.Errorf("[generic-executor] message %s: failed to send callback tx: %v", e.msgID, err3)
				} else {
					e.logger.Infof("[generic-executor] message %s: callback tx sent, txId: %s", e.msgID, callBackTxId)
				}
			}
		}
		return "", err
	}

	return txId, nil
}

// sendCrossChainTx 发送跨链交易（内部辅助函数）
func sendCrossChainTx(
	chainConfName, contractConfName, contractMethod string,
	kvs []*chainPb.KeyValuePair,
	svcCtx *svc.ServiceContext,
) (string, error) {
	txResp, err := svcCtx.ChainInteractiveServiceClient.CallContract(context.Background(), &chainCli.CallContractRequest{
		RequestId:      "cross-chain-service-generic-call",
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

// NewGenericExecutorWithReliability 创建带可靠性保障的通用执行器
func NewGenericExecutorWithReliability(
	svcCtx *svc.ServiceContext,
	router *MessageRouter,
	logger logx.Logger,
) *GenericExecutor {
	return &GenericExecutor{
		svcCtx: svcCtx,
		router: router,
		logger: logger,
	}
}

// CreateReliableGenericExecutor 创建带可靠性的通用内部执行器
// 在 message 包内自行实现可靠性逻辑，避免对 event 包的循环依赖
func CreateReliableGenericExecutor(
	svcCtx *svc.ServiceContext,
	logger logx.Logger,
	msgID string,
) CrossChainExecutor {
	rc := svcCtx.Config.ReliabilityConf
	subConf := svcCtx.Config.SubscribeConf

	// 如果未启用可靠性配置，返回基础执行器
	if !rc.EnableIdempotency && !rc.EnableRetry {
		return &genericInnerExecutor{
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
		logger.Errorf("[generic-executor] failed to create redis client: %v", err)
		return &genericInnerExecutor{
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

	inner := &genericInnerExecutor{
		svcCtx: svcCtx,
		logger: logger,
		msgID:  msgID,
	}

	return &reliableGenericExecutor{
		inner:       inner,
		idempotency: idempotChecker,
		taskStore:   taskStore,
		retry:       retryStrategy,
		logger:      logger,
		eventName:   msgID,
		idempotTTL:  idempotTTL,
	}
}

// reliableGenericExecutor 可靠的通用跨链交易执行器
// 在 message 包中实现，避免对 event 包的循环依赖
type reliableGenericExecutor struct {
	inner       CrossChainExecutor
	idempotency reliability.IdempotencyChecker
	taskStore   reliability.TaskStore
	retry       *reliability.RetryStrategy
	logger      logx.Logger
	eventName   string
	idempotTTL  time.Duration
}

// Execute 执行跨链交易（带可靠性保障）
func (e *reliableGenericExecutor) Execute(
	targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair,
) (string, error) {
	ctx := context.Background()

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
			txId, err = e.inner.Execute(targetChainName, targetContractName, method, kvs)
			return err
		})
	} else {
		txId, execErr = e.inner.Execute(targetChainName, targetContractName, method, kvs)
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
func (e *reliableGenericExecutor) ExecuteWithCallback(
	targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair,
	callbackMethod string, callbackKvsBuilder func(errMsg string) ([]*chainPb.KeyValuePair, error),
) (string, error) {
	ctx := context.Background()

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
				targetChainName, targetContractName,
				method, kvs, callbackMethod, callbackKvsBuilder,
			)
			return err
		})
	} else {
		txId, execErr = e.inner.ExecuteWithCallback(
			targetChainName, targetContractName,
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
	Execute(targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair) (string, error)
	ExecuteWithCallback(targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair,
		callbackMethod string, callbackKvsBuilder func(errMsg string) ([]*chainPb.KeyValuePair, error)) (string, error)
}
