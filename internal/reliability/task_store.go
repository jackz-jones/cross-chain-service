// Package reliability 可靠性保障机制
package reliability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// TaskState 跨链任务状态（字符串形式，与 message.MessageStatus 字面量保持一致，
// 避免维护两套状态机；旧的 int 枚举已废弃）。
type TaskState string

const (
	// TaskStatePending 待处理
	TaskStatePending TaskState = "pending"
	// TaskStateSubmitted 已提交
	TaskStateSubmitted TaskState = "submitted"
	// TaskStateConfirmed 已确认成功
	TaskStateConfirmed TaskState = "confirmed"
	// TaskStateFailed 失败
	TaskStateFailed TaskState = "failed"
	// TaskStateTimeout 已超时
	TaskStateTimeout TaskState = "timeout"
)

// String 返回状态字符串（保留原 API，方便日志/兼容）
func (s TaskState) String() string {
	if s == "" {
		return "unknown"
	}
	return string(s)
}

// IsTerminal 是否为终态（不再变更）
func (s TaskState) IsTerminal() bool {
	switch s {
	case TaskStateConfirmed, TaskStateFailed, TaskStateTimeout:
		return true
	default:
		return false
	}
}

// CrossChainTask 跨链任务
type CrossChainTask struct {
	// TaskID 任务唯一标识（应使用消息的 MessageID，保证跨执行器全局唯一）
	TaskID string `json:"taskId"`
	// EventKey 事件唯一标识（幂等 key）
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

// ErrTaskNotFound 任务不存在
var ErrTaskNotFound = errors.New("task not found")

// TaskStore 跨链任务持久化接口
type TaskStore interface {
	// Save 保存任务（终态自动从 pending 索引移除，非终态自动加入 pending 索引）
	Save(ctx context.Context, task *CrossChainTask) error
	// Get 获取任务；不存在时返回 (nil, nil)
	Get(ctx context.Context, taskID string) (*CrossChainTask, error)
	// UpdateState 更新任务状态
	UpdateState(ctx context.Context, taskID string, state TaskState, txID string, errMsg string) error
	// GetPendingTasks 获取待处理的任务列表
	GetPendingTasks(ctx context.Context) ([]*CrossChainTask, error)
}

// RedisKVClient Redis 字符串/集合操作接口
// 相较旧的 RedisHashClient，改用 SETEX（单 RTT）+ 索引集合来支撑 GetPendingTasks。
type RedisKVClient interface {
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, key string) error
	SAdd(ctx context.Context, key string, member interface{}) error
	SRem(ctx context.Context, key string, member interface{}) error
	SMembers(ctx context.Context, key string) ([]string, error)
}

// RedisTaskStore 基于 Redis 的任务持久化
type RedisTaskStore struct {
	client     RedisKVClient
	prefix     string
	pendingKey string
	ttl        time.Duration
}

// NewRedisTaskStore 创建 Redis 任务持久化
func NewRedisTaskStore(client RedisKVClient, prefix string, ttl time.Duration) *RedisTaskStore {
	if prefix == "" {
		prefix = "cross_chain:task:"
	}
	if ttl == 0 {
		ttl = 7 * 24 * time.Hour // 默认 7 天
	}
	return &RedisTaskStore{
		client:     client,
		prefix:     prefix,
		pendingKey: prefix + "pending",
		ttl:        ttl,
	}
}

// Save 保存任务（单次 RTT 完成写入 + TTL；同时维护 pending 索引集合）
func (s *RedisTaskStore) Save(ctx context.Context, task *CrossChainTask) error {
	key := s.prefix + task.TaskID
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}
	if err := s.client.Set(ctx, key, string(data), s.ttl); err != nil {
		return fmt.Errorf("failed to save task: %w", err)
	}
	// 维护 pending 索引：终态移除、非终态加入
	if task.State.IsTerminal() {
		if err := s.client.SRem(ctx, s.pendingKey, task.TaskID); err != nil {
			// 索引维护失败不影响主流程，仅上抛让调用方记日志
			return fmt.Errorf("failed to remove task from pending index: %w", err)
		}
	} else {
		if err := s.client.SAdd(ctx, s.pendingKey, task.TaskID); err != nil {
			return fmt.Errorf("failed to add task to pending index: %w", err)
		}
	}
	return nil
}

// Get 获取任务；不存在时返回 (nil, nil)
func (s *RedisTaskStore) Get(ctx context.Context, taskID string) (*CrossChainTask, error) {
	key := s.prefix + taskID
	data, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	if data == "" {
		return nil, nil
	}
	var task CrossChainTask
	if err := json.Unmarshal([]byte(data), &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task: %w", err)
	}
	return &task, nil
}

// UpdateState 更新任务状态
func (s *RedisTaskStore) UpdateState(
	ctx context.Context, taskID string, state TaskState, txID string, errMsg string,
) error {
	task, err := s.Get(ctx, taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}
	task.State = state
	if txID != "" {
		task.TxID = txID
	}
	task.ErrorMsg = errMsg
	task.UpdatedAt = time.Now()
	return s.Save(ctx, task)
}

// GetPendingTasks 获取待处理的任务列表（基于 pending 索引集合）
func (s *RedisTaskStore) GetPendingTasks(ctx context.Context) ([]*CrossChainTask, error) {
	ids, err := s.client.SMembers(ctx, s.pendingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending task ids: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	tasks := make([]*CrossChainTask, 0, len(ids))
	for _, id := range ids {
		t, err := s.Get(ctx, id)
		if err != nil {
			// 单条读失败不阻塞整体，跳过并继续
			continue
		}
		if t == nil {
			// 主 key 已经过期或被删除，顺便清理索引，避免长期悬挂
			_ = s.client.SRem(ctx, s.pendingKey, id)
			continue
		}
		if !t.State.IsTerminal() {
			tasks = append(tasks, t)
		} else {
			// 终态但索引未清理，补偿清理
			_ = s.client.SRem(ctx, s.pendingKey, id)
		}
	}
	return tasks, nil
}

// InMemoryTaskStore 内存版任务持久化（用于测试）
type InMemoryTaskStore struct {
	mu    sync.RWMutex
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.TaskID] = task
	return nil
}

// Get 获取任务
func (s *InMemoryTaskStore) Get(_ context.Context, taskID string) (*CrossChainTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[taskID]
	if !ok {
		return nil, nil
	}
	return task, nil
}

// UpdateState 更新任务状态
func (s *InMemoryTaskStore) UpdateState(
	_ context.Context, taskID string, state TaskState, txID string, errMsg string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}
	task.State = state
	if txID != "" {
		task.TxID = txID
	}
	task.ErrorMsg = errMsg
	task.UpdatedAt = time.Now()
	return nil
}

// GetPendingTasks 获取待处理的任务列表
func (s *InMemoryTaskStore) GetPendingTasks(_ context.Context) ([]*CrossChainTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var pending []*CrossChainTask
	for _, task := range s.tasks {
		if !task.State.IsTerminal() {
			pending = append(pending, task)
		}
	}
	return pending, nil
}
