package event

import (
	"errors"
	"testing"

	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
)

// mockHandler 用于测试 dispatcher 的 mock handler
type mockHandler struct {
	name      string
	called    bool
	lastEvent commonEvent.TradeGuardEvent
	returnErr error
}

func (m *mockHandler) eventName() string {
	return m.name
}

func (m *mockHandler) handleEvent(event commonEvent.TradeGuardEvent) error {
	m.called = true
	m.lastEvent = event
	return m.returnErr
}

func TestHandlerDispatcher_DispatchToCorrectHandler(t *testing.T) {
	logx.Disable()

	h1 := &mockHandler{name: "Event1"}
	h2 := &mockHandler{name: "Event2"}
	h3 := &mockHandler{name: "Event3"}

	dispatcher := newHandlerDispatcher([]handler{h1, h2, h3}, logx.WithContext(nil))

	event := commonEvent.TradeGuardEvent{
		EventName: "Event2",
		ChainType: "ethereum",
		EventData: []byte("test-data"),
	}

	err := dispatcher.dispatchTopicHandler(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if h1.called {
		t.Error("handler1 should not be called")
	}
	if !h2.called {
		t.Error("handler2 should be called")
	}
	if h3.called {
		t.Error("handler3 should not be called")
	}

	// 验证传递的事件数据正确
	if h2.lastEvent.EventName != "Event2" {
		t.Errorf("expected event name 'Event2', got '%s'", h2.lastEvent.EventName)
	}
	if h2.lastEvent.ChainType != "ethereum" {
		t.Errorf("expected chain type 'ethereum', got '%s'", h2.lastEvent.ChainType)
	}
}

func TestHandlerDispatcher_UnknownEventReturnsNil(t *testing.T) {
	logx.Disable()

	h1 := &mockHandler{name: "Event1"}
	dispatcher := newHandlerDispatcher([]handler{h1}, logx.WithContext(nil))

	event := commonEvent.TradeGuardEvent{
		EventName: "UnknownEvent",
	}

	err := dispatcher.dispatchTopicHandler(event)
	if err != nil {
		t.Fatalf("expected nil error for unknown event, got: %v", err)
	}

	if h1.called {
		t.Error("handler should not be called for unknown event")
	}
}

func TestHandlerDispatcher_PropagatesHandlerError(t *testing.T) {
	logx.Disable()

	expectedErr := errors.New("handler error")
	h1 := &mockHandler{name: "Event1", returnErr: expectedErr}
	dispatcher := newHandlerDispatcher([]handler{h1}, logx.WithContext(nil))

	event := commonEvent.TradeGuardEvent{
		EventName: "Event1",
	}

	err := dispatcher.dispatchTopicHandler(event)
	if err == nil {
		t.Fatal("expected error from handler")
	}
	if err.Error() != expectedErr.Error() {
		t.Errorf("expected error '%s', got '%s'", expectedErr.Error(), err.Error())
	}
}

func TestHandlerDispatcher_EmptyHandlers(t *testing.T) {
	logx.Disable()

	dispatcher := newHandlerDispatcher([]handler{}, logx.WithContext(nil))

	event := commonEvent.TradeGuardEvent{
		EventName: "AnyEvent",
	}

	err := dispatcher.dispatchTopicHandler(event)
	if err != nil {
		t.Fatalf("expected nil error for empty handlers, got: %v", err)
	}
}

func TestHandlerDispatcher_DuplicateEventNames(t *testing.T) {
	logx.Disable()

	h1 := &mockHandler{name: "Event1"}
	h2 := &mockHandler{name: "Event1"} // 重复事件名

	// 后注册的会覆盖先注册的
	dispatcher := newHandlerDispatcher([]handler{h1, h2}, logx.WithContext(nil))

	event := commonEvent.TradeGuardEvent{
		EventName: "Event1",
	}

	err := dispatcher.dispatchTopicHandler(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// h2 应该被调用（后注册覆盖）
	if !h2.called {
		t.Error("h2 should be called (last registered wins)")
	}
}
