// Package reliability 可靠性保障机制
package reliability

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// TaskState 跨链任务状态
type TaskState int

const (
	// TaskStatePending 待处理
	TaskStatePending TaskState = iota
	// TaskStateSubmitted 已提交
	TaskStateSubmitted
	// TaskStateConfirmed 已确认成功
	TaskStateConfirmed
	// TaskStateFailed 失败
	TaskStateFailed
)

// String 返回状态字符串
func (s TaskState) String() string {
	switch s {
	case TaskStatePending:
		return "pending"
	case TaskStateSubmitted:
		return "submitted"
	case TaskStateConfirmed:
		return "confirmed"
	case TaskStateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// CrossChainTask 跨链任务
type CrossChainTask struct {
	// TaskID 任务唯一标识
	TaskID string `json:"taskId"`
	// EventKey 事件唯一标识（OriginHash 或 TxId）
	EventKey string `json:"eventKey"`
	// SourceChain 源链名称
	SourceChain string `json:"sourceChain"`
	// TargetChain 目标链名称
	TargetChain string `json:"targetChain"`
	// ContractName 目标合约名称
	ContractName string `json:"contractName"`
	// Method 调用方法
	Method string `json:"method"`
	// State 当前状态
	State TaskState `json:"state"`
	// TxID 跨链交易 ID（提交后填充）
	TxID string `json:"txId,omitempty"`
	// ErrorMsg 错误信息
	ErrorMsg string `json:"errorMsg,omitempty"`
	// RetryCount 重试次数
	RetryCount int `json:"retryCount"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt 最后更新时间
	UpdatedAt time.Time `json:"updatedAt"`
}

// TaskStore 跨链任务持久化接口
type TaskStore interface {
	// Save 保存任务
	Save(ctx context.Context, task *CrossChainTask) error
	// Get 获取任务
	Get(ctx context.Context, taskID string) (*CrossChainTask, error)
	// UpdateState 更新任务状态
	UpdateState(ctx context.Context, taskID string, state TaskState, txID string, errMsg string) error
	// GetPendingTasks 获取待处理的任务列表
	GetPendingTasks(ctx context.Context) ([]*CrossChainTask, error)
}

// RedisTaskStore 基于 Redis 的任务持久化
type RedisTaskStore struct {
	client RedisHashClient
	prefix string
	ttl    time.Duration
}

// RedisHashClient Redis Hash 操作接口
type RedisHashClient interface {
	HSet(ctx context.Context, key string, field string, value interface{}) error
	HGet(ctx context.Context, key string, field string) (string, error)
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
}

// NewRedisTaskStore 创建 Redis 任务持久化
func NewRedisTaskStore(client RedisHashClient, prefix string, ttl time.Duration) *RedisTaskStore {
	if prefix == "" {
		prefix = "cross_chain:task:"
	}
	if ttl == 0 {
		ttl = 7 * 24 * time.Hour // 默认 7 天
	}
	return &RedisTaskStore{
		client: client,
		prefix: prefix,
		ttl:    ttl,
	}
}

// Save 保存任务
func (s *RedisTaskStore) Save(ctx context.Context, task *CrossChainTask) error {
	key := s.prefix + task.TaskID
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %v", err)
	}
	if err := s.client.HSet(ctx, key, "data", string(data)); err != nil {
		return fmt.Errorf("failed to save task: %v", err)
	}
	if err := s.client.Expire(ctx, key, s.ttl); err != nil {
		return fmt.Errorf("failed to set task TTL: %v", err)
	}
	return nil
}

// Get 获取任务
func (s *RedisTaskStore) Get(ctx context.Context, taskID string) (*CrossChainTask, error) {
	key := s.prefix + taskID
	data, err := s.client.HGet(ctx, key, "data")
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %v", err)
	}
	if data == "" {
		return nil, nil
	}
	var task CrossChainTask
	if err := json.Unmarshal([]byte(data), &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task: %v", err)
	}
	return &task, nil
}

// UpdateState 更新任务状态
func (s *RedisTaskStore) UpdateState(ctx context.Context, taskID string, state TaskState, txID string, errMsg string) error {
	task, err := s.Get(ctx, taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.State = state
	task.TxID = txID
	task.ErrorMsg = errMsg
	task.UpdatedAt = time.Now()
	return s.Save(ctx, task)
}

// GetPendingTasks 获取待处理的任务列表（简化实现）
func (s *RedisTaskStore) GetPendingTasks(_ context.Context) ([]*CrossChainTask, error) {
	// 注意：生产环境应使用 Redis Scan 或维护一个待处理任务列表
	// 此处为简化实现，实际使用时需要维护一个 pending 任务的 Set
	return nil, nil
}

// InMemoryTaskStore 内存版任务持久化（用于测试）
type InMemoryTaskStore struct {
	tasks map[string]*CrossChainTask
}

// NewInMemoryTaskStore 创建内存版任务持久化
func NewInMemoryTaskStore() *InMemoryTaskStore {
	return &InMemoryTaskStore{
		tasks: make(map[string]*CrossChainTask),
	}
}

// Save 保存任务
func (s *InMemoryTaskStore) Save(_ context.Context, task *CrossChainTask) error {
	s.tasks[task.TaskID] = task
	return nil
}

// Get 获取任务
func (s *InMemoryTaskStore) Get(_ context.Context, taskID string) (*CrossChainTask, error) {
	task, ok := s.tasks[taskID]
	if !ok {
		return nil, nil
	}
	return task, nil
}

// UpdateState 更新任务状态
func (s *InMemoryTaskStore) UpdateState(_ context.Context, taskID string, state TaskState, txID string, errMsg string) error {
	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.State = state
	task.TxID = txID
	task.ErrorMsg = errMsg
	task.UpdatedAt = time.Now()
	return nil
}

// GetPendingTasks 获取待处理的任务列表
func (s *InMemoryTaskStore) GetPendingTasks(_ context.Context) ([]*CrossChainTask, error) {
	var pending []*CrossChainTask
	for _, task := range s.tasks {
		if task.State == TaskStatePending || task.State == TaskStateSubmitted {
			pending = append(pending, task)
		}
	}
	return pending, nil
}
