package adapter

import (
	"fmt"
	"strings"
	"sync"
)

// Registry 适配器注册表
// 管理所有已注册的链适配器，支持按链类型查找
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]ChainAdapter
}

// 全局注册表实例
var globalRegistry = NewRegistry()

// NewRegistry 创建新的适配器注册表
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[string]ChainAdapter),
	}
}

// Register 注册链适配器
// chainType 会被转为小写存储，确保大小写不敏感
func (r *Registry) Register(adapter ChainAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[strings.ToLower(adapter.ChainType())] = adapter
}

// Get 根据链类型获取适配器
// chainType 大小写不敏感
func (r *Registry) Get(chainType string) (ChainAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	adapter, ok := r.adapters[strings.ToLower(chainType)]
	if !ok {
		return nil, fmt.Errorf("unsupported chain type: %s", chainType)
	}
	return adapter, nil
}

// GetAll 获取所有已注册的适配器
func (r *Registry) GetAll() map[string]ChainAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]ChainAdapter, len(r.adapters))
	for k, v := range r.adapters {
		result[k] = v
	}
	return result
}

// GlobalRegistry 获取全局注册表
func GlobalRegistry() *Registry {
	return globalRegistry
}

// RegisterAdapter 向全局注册表注册适配器（便捷函数）
func RegisterAdapter(adapter ChainAdapter) {
	globalRegistry.Register(adapter)
}

// GetAdapter 从全局注册表获取适配器（便捷函数）
func GetAdapter(chainType string) (ChainAdapter, error) {
	return globalRegistry.Get(chainType)
}
