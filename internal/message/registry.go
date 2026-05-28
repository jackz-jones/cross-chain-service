package message

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/zeromicro/go-zero/core/logx"
)

// HandlerRegistry 事件处理器注册表
// 管理所有已注册的 GenericEventHandler，支持按名称查找
type HandlerRegistry struct {
	mu       sync.RWMutex
	handlers map[string]GenericEventHandler
	logger   logx.Logger
}

// 全局注册表实例
var globalHandlerRegistry *HandlerRegistry

// initGlobalRegistry 初始化全局注册表（延迟初始化，在第一次使用时调用）
func initGlobalRegistry() {
	if globalHandlerRegistry == nil {
		globalHandlerRegistry = NewHandlerRegistry(logx.WithContext(context.Background()))
	}
}

// NewHandlerRegistry 创建新的事件处理器注册表
func NewHandlerRegistry(logger logx.Logger) *HandlerRegistry {
	return &HandlerRegistry{
		handlers: make(map[string]GenericEventHandler),
		logger:   logger,
	}
}

// Register 注册事件处理器
func (r *HandlerRegistry) Register(handler GenericEventHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := strings.ToLower(handler.Name())
	if _, exists := r.handlers[name]; exists {
		r.logger.Errorf("[registry] handler '%s' already registered, overwriting", handler.Name())
	}
	r.handlers[name] = handler
	r.logger.Infof("[registry] registered handler: %s", handler.Name())
}

// Get 根据名称获取事件处理器
func (r *HandlerRegistry) Get(name string) (GenericEventHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, ok := r.handlers[strings.ToLower(name)]
	return handler, ok
}

// GetAll 获取所有已注册的处理器
func (r *HandlerRegistry) GetAll() map[string]GenericEventHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]GenericEventHandler, len(r.handlers))
	for k, v := range r.handlers {
		result[k] = v
	}
	return result
}

// GlobalHandlerRegistry 获取全局处理器注册表
func GlobalHandlerRegistry() *HandlerRegistry {
	initGlobalRegistry()
	return globalHandlerRegistry
}

// RegisterHandler 向全局注册表注册处理器（便捷函数）
func RegisterHandler(handler GenericEventHandler) {
	globalHandlerRegistry.Register(handler)
}

// GetHandler 从全局注册表获取处理器（便捷函数）
func GetHandler(name string) (GenericEventHandler, error) {
	handler, ok := globalHandlerRegistry.Get(name)
	if !ok {
		return nil, fmt.Errorf("handler not found: %s", name)
	}
	return handler, nil
}
