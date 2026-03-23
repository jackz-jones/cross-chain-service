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

// handlerDispatcher 事件分发器
type handlerDispatcher struct {
	handlersMap map[string]handler
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

// dispatchTopicHandler 事件分发
func (h *handlerDispatcher) dispatchTopicHandler(event event.TradeGuardEvent) error {
	if eventHandler, exist := h.handlersMap[event.EventName]; exist {
		return eventHandler.handleEvent(event)
	}

	return nil
}
