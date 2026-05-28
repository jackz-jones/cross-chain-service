package message

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	chainPb "github.com/jackz-jones/blockchain-interactive-service/pb"
	"github.com/jackz-jones/cross-chain-service/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

// TimeoutScanner 超时扫描器
// 定期检查未确认且已超时的跨链消息，触发回调或标记失败
type TimeoutScanner struct {
	svcCtx   *svc.ServiceContext
	logger   logx.Logger
	executor CrossChainExecutor
	mu       sync.Mutex
	running  bool
	cancel   context.CancelFunc

	// scanInterval 扫描间隔
	scanInterval time.Duration

	// pendingMessages 待确认消息存储（内存缓存，后续可切换为 Redis）
	pendingMessages map[string]*CrossChainMessage
}

// NewTimeoutScanner 创建超时扫描器
func NewTimeoutScanner(
	svcCtx *svc.ServiceContext,
	logger logx.Logger,
	executor CrossChainExecutor,
	scanInterval time.Duration,
) *TimeoutScanner {
	if scanInterval == 0 {
		scanInterval = 30 * time.Second
	}
	return &TimeoutScanner{
		svcCtx:          svcCtx,
		logger:          logger,
		executor:        executor,
		scanInterval:    scanInterval,
		pendingMessages: make(map[string]*CrossChainMessage),
	}
}

// Start 启动超时扫描器
func (s *TimeoutScanner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.running = true

	go s.scanLoop(ctx)
	s.logger.Info("[timeout-scanner] started")
}

// Stop 停止超时扫描器
func (s *TimeoutScanner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.cancel()
	s.running = false
	s.logger.Info("[timeout-scanner] stopped")
}

// AddMessage 添加待监控的消息
func (s *TimeoutScanner) AddMessage(msg *CrossChainMessage) {
	// 即发即忘消息不需要超时检测
	if msg.IsFireAndForget() {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.pendingMessages[msg.MessageID] = msg
	s.logger.Infof("[timeout-scanner] added message %s for timeout monitoring (timeout: %d)",
		msg.MessageID, msg.Timeout)
}

// RemoveMessage 移除已确认的消息
func (s *TimeoutScanner) RemoveMessage(messageID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.pendingMessages, messageID)
}

// scanLoop 扫描循环
func (s *TimeoutScanner) scanLoop(ctx context.Context) {
	ticker := time.NewTicker(s.scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.scan()
		}
	}
}

// scan 执行一次超时扫描
func (s *TimeoutScanner) scan() {
	s.mu.Lock()
	messages := make(map[string]*CrossChainMessage)
	for k, v := range s.pendingMessages {
		messages[k] = v
	}
	s.mu.Unlock()

	now := time.Now().Unix()

	for messageID, msg := range messages {
		// 检查是否超时
		if msg.Timeout == 0 || now <= msg.Timeout {
			continue
		}

		s.logger.Infof("[timeout-scanner] message %s has timed out (timeout: %d, now: %d)",
			messageID, msg.Timeout, now)

		// 处理超时消息
		s.handleTimeout(msg)

		// 从待确认列表中移除
		s.mu.Lock()
		delete(s.pendingMessages, messageID)
		s.mu.Unlock()
	}
}

// handleTimeout 处理超时消息
func (s *TimeoutScanner) handleTimeout(msg *CrossChainMessage) {
	msg.Status = StatusTimeout
	msg.ErrorMessage = "message timed out"

	// 如果有回调方法，执行回调
	if msg.HasCallback() {
		s.executeTimeoutCallback(msg)
	} else {
		// 无回调，仅记录日志
		s.logger.Infof("[timeout-scanner] message %s timed out without callback, marking as timeout",
			msg.MessageID)
	}
}

// executeTimeoutCallback 执行超时回调
func (s *TimeoutScanner) executeTimeoutCallback(msg *CrossChainMessage) {
	s.logger.Infof("[timeout-scanner] executing timeout callback for message %s, method: %s",
		msg.MessageID, msg.CallbackMethod)

	// 解析回调参数
	var callbackKvs []*chainPb.KeyValuePair
	if err := json.Unmarshal(msg.CallbackPayload, &callbackKvs); err != nil {
		s.logger.Errorf("[timeout-scanner] failed to unmarshal callback payload for message %s: %v",
			msg.MessageID, err)
		return
	}

	// 构建包含超时错误信息的回调参数
	timeoutErrMsg := fmt.Sprintf("cross-chain message timed out (messageId: %s)", msg.MessageID)

	callbackKvsBuilder := func(innerErrMsg string) ([]*chainPb.KeyValuePair, error) {
		return buildCallbackKvsWithError(callbackKvs, timeoutErrMsg), nil
	}

	// 执行回调（超时回调不需要再设置回调）
	_, err := s.executor.ExecuteWithCallback(
		msg.SourceChain,    // 回调到源链
		msg.SourceContract, // 回调到源合约
		msg.CallbackMethod,
		callbackKvs,
		"",
		callbackKvsBuilder,
	)

	if err != nil {
		s.logger.Errorf("[timeout-scanner] failed to execute timeout callback for message %s: %v",
			msg.MessageID, err)
	} else {
		s.logger.Infof("[timeout-scanner] successfully executed timeout callback for message %s",
			msg.MessageID)
	}
}

// buildCallbackKvsWithError 在回调参数中注入错误信息
func buildCallbackKvsWithError(kvs []*chainPb.KeyValuePair, errMsg string) []*chainPb.KeyValuePair {
	result := make([]*chainPb.KeyValuePair, len(kvs))
	copy(result, kvs)

	// 添加错误信息字段
	result = append(result, &chainPb.KeyValuePair{
		Key:   "errorMsg",
		Value: []byte(errMsg),
	})

	return result
}
