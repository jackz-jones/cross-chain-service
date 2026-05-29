// Package metrics 可观测性指标（预留扩展模块，当前未在主代码中启用）
package metrics

import (
	"sort"
	"sync"
	"time"
)

// MetricType 指标类型
type MetricType int

const (
	MetricTypeCounter   MetricType = iota // 计数器
	MetricTypeGauge                       // 仪表盘
	MetricTypeHistogram                   // 直方图
)

// Labels 指标标签
type Labels map[string]string

// Metric 指标接口
type Metric interface {
	// Inc 计数器+1
	Inc(labels Labels)
	// Add 计数器+N
	Add(labels Labels, value float64)
	// Set 设置仪表盘值
	Set(labels Labels, value float64)
	// Observe 记录直方图观测值
	Observe(labels Labels, value float64)
}

// Collector 指标收集器接口
type Collector interface {
	// Counter 获取或创建计数器
	Counter(name, help string) Metric
	// Gauge 获取或创建仪表盘
	Gauge(name, help string) Metric
	// Histogram 获取或创建直方图
	Histogram(name, help string, buckets []float64) Metric
}

// --- 跨链服务预定义指标 ---

// CrossChainMetrics 跨链服务指标集合
type CrossChainMetrics struct {
	collector Collector

	// EventReceived 接收到的事件总数
	EventReceived Metric
	// EventProcessed 成功处理的事件总数
	EventProcessed Metric
	// EventFailed 处理失败的事件总数
	EventFailed Metric
	// EventDuplicate 重复事件总数
	EventDuplicate Metric
	// CrossChainTxSubmitted 提交的跨链交易总数
	CrossChainTxSubmitted Metric
	// CrossChainTxDuration 跨链交易耗时
	CrossChainTxDuration Metric
	// RetryCount 重试次数
	RetryCount Metric
	// ActiveChains 活跃链数量
	ActiveChains Metric
}

// NewCrossChainMetrics 创建跨链服务指标集合
func NewCrossChainMetrics(collector Collector) *CrossChainMetrics {
	m := &CrossChainMetrics{collector: collector}

	m.EventReceived = collector.Counter(
		"cross_chain_event_received_total",
		"接收到的事件总数",
	)
	m.EventProcessed = collector.Counter(
		"cross_chain_event_processed_total",
		"成功处理的事件总数",
	)
	m.EventFailed = collector.Counter(
		"cross_chain_event_failed_total",
		"处理失败的事件总数",
	)
	m.EventDuplicate = collector.Counter(
		"cross_chain_event_duplicate_total",
		"重复事件总数",
	)
	m.CrossChainTxSubmitted = collector.Counter(
		"cross_chain_tx_submitted_total",
		"提交的跨链交易总数",
	)
	m.CrossChainTxDuration = collector.Histogram(
		"cross_chain_tx_duration_seconds",
		"跨链交易耗时（秒）",
		[]float64{0.1, 0.5, 1, 2, 5, 10, 30},
	)
	m.RetryCount = collector.Counter(
		"cross_chain_retry_total",
		"重试次数",
	)
	m.ActiveChains = collector.Gauge(
		"cross_chain_active_chains",
		"活跃链数量",
	)

	return m
}

// RecordEventReceived 记录接收到事件
func (m *CrossChainMetrics) RecordEventReceived(eventName, chainName string) {
	m.EventReceived.Inc(Labels{"event": eventName, "chain": chainName})
}

// RecordEventProcessed 记录事件处理成功
func (m *CrossChainMetrics) RecordEventProcessed(eventName, chainName string) {
	m.EventProcessed.Inc(Labels{"event": eventName, "chain": chainName})
}

// RecordEventFailed 记录事件处理失败
func (m *CrossChainMetrics) RecordEventFailed(eventName, chainName, reason string) {
	m.EventFailed.Inc(Labels{"event": eventName, "chain": chainName, "reason": reason})
}

// RecordEventDuplicate 记录重复事件
func (m *CrossChainMetrics) RecordEventDuplicate(eventName, chainName string) {
	m.EventDuplicate.Inc(Labels{"event": eventName, "chain": chainName})
}

// RecordCrossChainTx 记录跨链交易
func (m *CrossChainMetrics) RecordCrossChainTx(sourceChain, targetChain, method string, duration time.Duration) {
	labels := Labels{"source": sourceChain, "target": targetChain, "method": method}
	m.CrossChainTxSubmitted.Inc(labels)
	m.CrossChainTxDuration.Observe(labels, duration.Seconds())
}

// RecordRetry 记录重试
func (m *CrossChainMetrics) RecordRetry(eventName, chainName string) {
	m.RetryCount.Inc(Labels{"event": eventName, "chain": chainName})
}

// SetActiveChains 设置活跃链数量
func (m *CrossChainMetrics) SetActiveChains(count int) {
	m.ActiveChains.Set(Labels{}, float64(count))
}

// --- InMemory 实现（用于测试） ---

// InMemoryCollector 内存指标收集器
type InMemoryCollector struct {
	mu      sync.RWMutex
	metrics map[string]*InMemoryMetric
}

// NewInMemoryCollector 创建内存指标收集器
func NewInMemoryCollector() *InMemoryCollector {
	return &InMemoryCollector{
		metrics: make(map[string]*InMemoryMetric),
	}
}

// Counter 获取或创建计数器
func (c *InMemoryCollector) Counter(name, help string) Metric {
	return c.getOrCreate(name)
}

// Gauge 获取或创建仪表盘
func (c *InMemoryCollector) Gauge(name, help string) Metric {
	return c.getOrCreate(name)
}

// Histogram 获取或创建直方图
func (c *InMemoryCollector) Histogram(name, help string, buckets []float64) Metric {
	return c.getOrCreate(name)
}

func (c *InMemoryCollector) getOrCreate(name string) *InMemoryMetric {
	c.mu.Lock()
	defer c.mu.Unlock()
	if m, ok := c.metrics[name]; ok {
		return m
	}
	m := &InMemoryMetric{values: make(map[string]float64)}
	c.metrics[name] = m
	return m
}

// GetValue 获取指标值（用于测试断言）
func (c *InMemoryCollector) GetValue(name string, labels Labels) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m, ok := c.metrics[name]
	if !ok {
		return 0
	}
	return m.GetValue(labels)
}

// InMemoryMetric 内存指标
type InMemoryMetric struct {
	mu     sync.RWMutex
	values map[string]float64
}

func (m *InMemoryMetric) labelKey(labels Labels) string {
	if len(labels) == 0 {
		return "__default__"
	}
	// 排序 key 确保一致性
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	key := ""
	for _, k := range keys {
		key += k + "=" + labels[k] + ","
	}
	return key
}

// Inc 计数器+1
func (m *InMemoryMetric) Inc(labels Labels) {
	m.Add(labels, 1)
}

// Add 计数器+N
func (m *InMemoryMetric) Add(labels Labels, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := m.labelKey(labels)
	m.values[key] += value
}

// Set 设置仪表盘值
func (m *InMemoryMetric) Set(labels Labels, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := m.labelKey(labels)
	m.values[key] = value
}

// Observe 记录直方图观测值
func (m *InMemoryMetric) Observe(labels Labels, value float64) {
	m.Add(labels, value)
}

// GetValue 获取指标值
func (m *InMemoryMetric) GetValue(labels Labels) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := m.labelKey(labels)
	return m.values[key]
}
