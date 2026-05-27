// Package metrics 可观测性指标
package metrics

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// HealthStatus 健康状态
type HealthStatus string

const (
	HealthStatusUp   HealthStatus = "UP"
	HealthStatusDown HealthStatus = "DOWN"
)

// HealthCheck 健康检查组件
type HealthCheck struct {
	mu     sync.RWMutex
	checks map[string]CheckFunc
}

// CheckFunc 健康检查函数
type CheckFunc func() error

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    HealthStatus      `json:"status"`
	Timestamp string            `json:"timestamp"`
	Checks    map[string]string `json:"checks"`
}

// NewHealthCheck 创建健康检查组件
func NewHealthCheck() *HealthCheck {
	return &HealthCheck{
		checks: make(map[string]CheckFunc),
	}
}

// Register 注册健康检查项
func (h *HealthCheck) Register(name string, check CheckFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = check
}

// Check 执行所有健康检查
func (h *HealthCheck) Check() *HealthResponse {
	h.mu.RLock()
	defer h.mu.RUnlock()

	resp := &HealthResponse{
		Status:    HealthStatusUp,
		Timestamp: time.Now().Format(time.RFC3339),
		Checks:    make(map[string]string),
	}

	for name, check := range h.checks {
		if err := check(); err != nil {
			resp.Status = HealthStatusDown
			resp.Checks[name] = "DOWN: " + err.Error()
		} else {
			resp.Checks[name] = "UP"
		}
	}

	return resp
}

// IsHealthy 是否健康
func (h *HealthCheck) IsHealthy() bool {
	resp := h.Check()
	return resp.Status == HealthStatusUp
}

// HTTPHandler 返回 HTTP 健康检查处理器
func (h *HealthCheck) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := h.Check()

		w.Header().Set("Content-Type", "application/json")
		if resp.Status == HealthStatusDown {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		_ = json.NewEncoder(w).Encode(resp)
	}
}
