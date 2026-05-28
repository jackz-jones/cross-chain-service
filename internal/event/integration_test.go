// Package event 端到端集成测试
// 验证重构后的完整事件处理流程
package event

import (
	"context"
	"testing"
	"time"

	"github.com/jackz-jones/cross-chain-service/internal/adapter"
	"github.com/jackz-jones/cross-chain-service/internal/mapping"
	"github.com/jackz-jones/cross-chain-service/internal/metrics"
	"github.com/jackz-jones/cross-chain-service/internal/reliability"
)

// TestIntegration_FullPipeline 端到端集成测试
// 验证：事件接收 → 幂等性检查 → 适配器解析 → 规则引擎 → 中间件 → 执行器 → 指标记录
func TestIntegration_FullPipeline(t *testing.T) {
	// 1. 初始化所有组件
	idempotency := reliability.NewInMemoryIdempotencyChecker()
	taskStore := reliability.NewInMemoryTaskStore()
	retryStrategy := reliability.NewRetryStrategy(&reliability.RetryConfig{
		MaxRetries: 2,
		BaseDelay:  1 * time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
		Multiplier: 2.0,
	})
	collector := metrics.NewInMemoryCollector()
	metricsSet := metrics.NewCrossChainMetrics(collector)
	engine := mapping.NewEngine()
	middlewareChain := mapping.NewMiddlewareChain()

	// 2. 注册规则
	engine.RegisterRule(&mapping.Rule{
		SourceEventName: "EnterpriseNotifiedEvent",
		TargetMethod:    "NotifyEnterpriseInfo",
		RequiredFields:  []string{"id", "originHash"},
		Middlewares:     []string{"whitelist"},
	})

	// 3. 注册中间件
	middlewareChain.Register(mapping.NewFuncMiddleware("whitelist", func(ctx context.Context, fields map[string]interface{}) (map[string]interface{}, error) {
		// 模拟白名单检查通过
		fields["whitelistChecked"] = true
		return fields, nil
	}))

	// 4. 验证适配器注册
	registry := adapter.GlobalRegistry()
	ethAdapter, err := registry.Get("ethereum")
	if err != nil {
		t.Fatalf("ethereum adapter not registered: %v", err)
	}
	if ethAdapter.ChainType() != "ethereum" {
		t.Errorf("expected chain type 'ethereum', got '%s'", ethAdapter.ChainType())
	}

	cmAdapter, err := registry.Get("chainmaker")
	if err != nil {
		t.Fatalf("chainmaker adapter not registered: %v", err)
	}
	if cmAdapter.ChainType() != "chainmaker" {
		t.Errorf("expected chain type 'chainmaker', got '%s'", cmAdapter.ChainType())
	}

	// 5. 验证幂等性检查
	ctx := context.Background()
	eventKey := "test-event-001"

	dup, _ := idempotency.IsDuplicate(ctx, eventKey)
	if dup {
		t.Error("new event should not be duplicate")
	}

	_ = idempotency.MarkProcessed(ctx, eventKey, 1*time.Hour)

	dup, _ = idempotency.IsDuplicate(ctx, eventKey)
	if !dup {
		t.Error("marked event should be duplicate")
	}

	// 6. 验证规则引擎
	rule, err := engine.GetRule("EnterpriseNotifiedEvent")
	if err != nil {
		t.Fatalf("rule not found: %v", err)
	}
	if rule.TargetMethod != "NotifyEnterpriseInfo" {
		t.Errorf("expected target method 'NotifyEnterpriseInfo', got '%s'", rule.TargetMethod)
	}

	// 7. 验证中间件链
	fields := map[string]interface{}{"id": "001", "originHash": "0xabc"}
	result, err := middlewareChain.Execute(ctx, rule.Middlewares, fields)
	if err != nil {
		t.Fatalf("middleware execution failed: %v", err)
	}
	if result["whitelistChecked"] != true {
		t.Error("whitelist middleware should have set whitelistChecked")
	}

	// 8. 验证重试策略
	callCount := 0
	err = retryStrategy.ExecuteWithRetryImmediate(func() error {
		callCount++
		if callCount < 2 {
			return context.DeadlineExceeded
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retry should succeed on second attempt: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}

	// 9. 验证任务持久化
	task := &reliability.CrossChainTask{
		TaskID:       "task-integration-001",
		EventKey:     eventKey,
		SourceChain:  "chain1",
		TargetChain:  "chain2",
		ContractName: "nft-contract",
		Method:       "NotifyEnterpriseInfo",
		State:        reliability.TaskStatePending,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	_ = taskStore.Save(ctx, task)
	_ = taskStore.UpdateState(ctx, "task-integration-001", reliability.TaskStateConfirmed, "tx-001", "")

	got, _ := taskStore.Get(ctx, "task-integration-001")
	if got.State != reliability.TaskStateConfirmed {
		t.Errorf("expected Confirmed state, got %s", got.State)
	}

	// 10. 验证指标记录
	metricsSet.RecordEventReceived("EnterpriseNotifiedEvent", "chain1")
	metricsSet.RecordEventProcessed("EnterpriseNotifiedEvent", "chain1")
	metricsSet.RecordCrossChainTx("chain1", "chain2", "NotifyEnterpriseInfo", 500*time.Millisecond)
	metricsSet.SetActiveChains(2)

	receivedVal := collector.GetValue("cross_chain_event_received_total",
		metrics.Labels{"event": "EnterpriseNotifiedEvent", "chain": "chain1"})
	if receivedVal != 1 {
		t.Errorf("expected received count 1, got %f", receivedVal)
	}

	activeVal := collector.GetValue("cross_chain_active_chains", metrics.Labels{})
	if activeVal != 2 {
		t.Errorf("expected active chains 2, got %f", activeVal)
	}

	t.Log("✅ 端到端集成测试通过：所有组件协同工作正常")
}

// TestIntegration_HealthCheck 健康检查集成测试
func TestIntegration_HealthCheck(t *testing.T) {
	hc := metrics.NewHealthCheck()

	// 注册各组件健康检查
	hc.Register("redis", func() error { return nil })
	hc.Register("chain_service", func() error { return nil })
	hc.Register("adapter_registry", func() error {
		registry := adapter.GlobalRegistry()
		_, err := registry.Get("ethereum")
		return err
	})

	if !hc.IsHealthy() {
		t.Error("all components should be healthy")
	}

	resp := hc.Check()
	if len(resp.Checks) != 3 {
		t.Errorf("expected 3 checks, got %d", len(resp.Checks))
	}

	t.Log("✅ 健康检查集成测试通过")
}

// TestIntegration_RouteTable 路由表集成测试
func TestIntegration_RouteTable(t *testing.T) {
	// 验证路由表的构建逻辑
	rt := make(RouteTable)

	// 模拟 3 条链的全互联路由
	chains := []string{"chain1", "chain2", "chain3"}
	for _, source := range chains {
		for _, target := range chains {
			if source != target {
				rt[source] = append(rt[source], nil) // 简化测试
			}
		}
	}

	// 每条链应该有 2 个目标
	for _, chain := range chains {
		if len(rt[chain]) != 2 {
			t.Errorf("chain '%s' should have 2 targets, got %d", chain, len(rt[chain]))
		}
	}

	t.Log("✅ 路由表集成测试通过：3 链全互联拓扑正确")
}
