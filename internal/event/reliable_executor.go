// Package event 合约事件处理
package event

import (
	"context"
	"time"

	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	"github.com/jackz-jones/cross-chain-service/internal/reliability"
	"github.com/zeromicro/go-zero/core/logx"
)

// ReliableCrossChainExecutor 可靠的跨链交易执行器
// 在 DefaultCrossChainExecutor 基础上增加幂等性检查和重试机制
type ReliableCrossChainExecutor struct {
	inner        CrossChainExecutor
	idempotency  reliability.IdempotencyChecker
	taskStore    reliability.TaskStore
	retry        *reliability.RetryStrategy
	logger       logx.Logger
	eventName    string
	idempotTTL   time.Duration
}

// ReliableExecutorOption 可靠执行器配置选项
type ReliableExecutorOption func(*ReliableCrossChainExecutor)

// WithIdempotency 设置幂等性检查器
func WithIdempotency(checker reliability.IdempotencyChecker, ttl time.Duration) ReliableExecutorOption {
	return func(e *ReliableCrossChainExecutor) {
		e.idempotency = checker
		e.idempotTTL = ttl
	}
}

// WithTaskStore 设置任务持久化
func WithTaskStore(store reliability.TaskStore) ReliableExecutorOption {
	return func(e *ReliableCrossChainExecutor) {
		e.taskStore = store
	}
}

// WithRetry 设置重试策略
func WithRetry(strategy *reliability.RetryStrategy) ReliableExecutorOption {
	return func(e *ReliableCrossChainExecutor) {
		e.retry = strategy
	}
}

// NewReliableCrossChainExecutor 创建可靠的跨链交易执行器
func NewReliableCrossChainExecutor(
	inner CrossChainExecutor,
	logger logx.Logger,
	eventName string,
	opts ...ReliableExecutorOption,
) *ReliableCrossChainExecutor {
	e := &ReliableCrossChainExecutor{
		inner:      inner,
		logger:     logger,
		eventName:  eventName,
		idempotTTL: 24 * time.Hour, // 默认 24 小时
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Execute 执行跨链交易（带可靠性保障）
func (e *ReliableCrossChainExecutor) Execute(targetChainName, targetContractName, method string, kvs []*chainPb.KeyValuePair) (string, error) {
	ctx := context.Background()

	// 1. 幂等性检查（如果配置了）
	eventKey := targetChainName + ":" + targetContractName + ":" + method
	if e.idempotency != nil {
		dup, err := e.idempotency.IsDuplicate(ctx, eventKey)
		if err != nil {
			e.logger.Errorf("[%s] idempotency check error: %v", e.eventName, err)
			// 检查失败不阻塞，继续执行
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

	// 3. 标记已处理（如果配置了幂等性且执行成功）
	if execErr == nil && e.idempotency != nil {
		if err := e.idempotency.MarkProcessed(ctx, eventKey, e.idempotTTL); err != nil {
			e.logger.Errorf("[%s] failed to mark event as processed: %v", e.eventName, err)
		}
	}

	// 4. 记录任务状态（如果配置了任务持久化）
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
func (e *ReliableCrossChainExecutor) ExecuteWithCallback(
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
			txId, err = e.inner.ExecuteWithCallback(targetChainName, targetContractName, method, kvs, callbackMethod, callbackKvsBuilder)
			return err
		})
	} else {
		txId, execErr = e.inner.ExecuteWithCallback(targetChainName, targetContractName, method, kvs, callbackMethod, callbackKvsBuilder)
	}

	// 3. 标记已处理
	if execErr == nil && e.idempotency != nil {
		if err := e.idempotency.MarkProcessed(ctx, eventKey, e.idempotTTL); err != nil {
			e.logger.Errorf("[%s] failed to mark event as processed: %v", e.eventName, err)
		}
	}

	return txId, execErr
}
