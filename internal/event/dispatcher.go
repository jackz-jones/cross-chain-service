package event

import (
	"github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
)

// 事件处理器接口定义
type handler interface {
	eventName() string
	handleEvent(event event.TradeGuardEvent) error
}

// DropCallback 当事件未命中任何 handler 时被调用，可用于埋点/指标上报。
// 若为 nil 则忽略。
type DropCallback func(event event.TradeGuardEvent)

// handlerDispatcher 事件分发器
type handlerDispatcher struct {
	handlersMap map[string]handler
	onDrop      DropCallback
	logx.Logger
}

// newHandlerDispatcher 实例化handler分发器
func newHandlerDispatcher(handlers []handler, logger logx.Logger) *handlerDispatcher {
	handlersMap := make(map[string]handler, len(handlers))
	for _, h := range handlers {
		handlersMap[h.eventName()] = h
	}
	return &handlerDispatcher{
		handlersMap: handlersMap,
		Logger:      logger,
	}
}

// SetDropCallback 设置未命中事件的回调（例如上报 metrics）。
// 传入 nil 可清除回调。
func (h *handlerDispatcher) SetDropCallback(cb DropCallback) {
	h.onDrop = cb
}

// dispatchTopicHandler 事件分发
func (h *handlerDispatcher) dispatchTopicHandler(event event.TradeGuardEvent) error {
	if eventHandler, exist := h.handlersMap[event.EventName]; exist {
		return eventHandler.handleEvent(event)
	}

	// 未命中任何 handler：记录 Warn 日志，便于运维排查漏配路由
	h.Logger.Errorf("[dispatcher] no handler registered for event: name=%s chainType=%s chain=%s contractType=%s contract=%s",
		event.EventName, event.ChainType, event.ChainName, event.ContractType, event.ContractName)
	if h.onDrop != nil {
		h.onDrop(event)
	}
	return nil
}
