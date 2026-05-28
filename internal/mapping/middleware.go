// Package mapping 事件-动作映射规则引擎
package mapping

import (
	"context"
	"fmt"
)

// Middleware 中间件接口
// 在事件处理流程中执行自定义逻辑（如白名单检查、文件通知等）
type Middleware interface {
	// Name 返回中间件名称
	Name() string

	// Execute 执行中间件逻辑
	// ctx: 上下文
	// fields: 事件字段
	// 返回修改后的字段和错误
	Execute(ctx context.Context, fields map[string]interface{}) (map[string]interface{}, error)
}

// MiddlewareChain 中间件链
type MiddlewareChain struct {
	middlewares map[string]Middleware
}

// NewMiddlewareChain 创建中间件链
func NewMiddlewareChain() *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: make(map[string]Middleware),
	}
}

// Register 注册中间件
func (c *MiddlewareChain) Register(mw Middleware) {
	c.middlewares[mw.Name()] = mw
}

// Execute 按名称列表顺序执行中间件
func (c *MiddlewareChain) Execute(
	ctx context.Context, names []string, fields map[string]interface{},
) (map[string]interface{}, error) {
	current := fields
	for _, name := range names {
		mw, ok := c.middlewares[name]
		if !ok {
			return nil, fmt.Errorf("middleware not found: %s", name)
		}
		var err error
		current, err = mw.Execute(ctx, current)
		if err != nil {
			return nil, fmt.Errorf("middleware '%s' failed: %v", name, err)
		}
	}
	return current, nil
}

// Get 获取中间件
func (c *MiddlewareChain) Get(name string) (Middleware, bool) {
	mw, ok := c.middlewares[name]
	return mw, ok
}

// FuncMiddleware 函数式中间件（便捷创建）
type FuncMiddleware struct {
	name string
	fn   func(ctx context.Context, fields map[string]interface{}) (map[string]interface{}, error)
}

// NewFuncMiddleware 创建函数式中间件
func NewFuncMiddleware(
	name string,
	fn func(ctx context.Context, fields map[string]interface{}) (map[string]interface{}, error),
) *FuncMiddleware {
	return &FuncMiddleware{name: name, fn: fn}
}

// Name 返回中间件名称
func (m *FuncMiddleware) Name() string {
	return m.name
}

// Execute 执行中间件逻辑
func (m *FuncMiddleware) Execute(ctx context.Context, fields map[string]interface{}) (map[string]interface{}, error) {
	return m.fn(ctx, fields)
}
