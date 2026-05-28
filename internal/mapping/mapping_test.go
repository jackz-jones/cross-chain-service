package mapping

import (
	"context"
	"errors"
	"testing"
)

const (
	testPrefix = "pre_"
	testSuffix = "_suf"
)

// --- 规则引擎测试 ---

func TestEngine_RegisterAndGetRule(t *testing.T) {
	engine := NewEngine()

	rule := &Rule{
		SourceEventName: "EnterpriseNotifiedEvent",
		TargetMethod:    "NotifyEnterpriseInfo",
		CallbackMethod:  "Callback",
		RequiredFields:  []string{"id", "originHash"},
	}

	engine.RegisterRule(rule)

	got, err := engine.GetRule("EnterpriseNotifiedEvent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.TargetMethod != "NotifyEnterpriseInfo" {
		t.Errorf("expected TargetMethod 'NotifyEnterpriseInfo', got '%s'", got.TargetMethod)
	}
}

func TestEngine_GetRule_NotFound(t *testing.T) {
	engine := NewEngine()

	_, err := engine.GetRule("NonExistentEvent")
	if err == nil {
		t.Fatal("expected error for non-existent rule")
	}
}

func TestEngine_HasRule(t *testing.T) {
	engine := NewEngine()
	engine.RegisterRule(&Rule{SourceEventName: "Event1"})

	if !engine.HasRule("Event1") {
		t.Error("expected HasRule to return true for registered event")
	}
	if engine.HasRule("Event2") {
		t.Error("expected HasRule to return false for unregistered event")
	}
}

func TestEngine_RegisterRules_Batch(t *testing.T) {
	engine := NewEngine()
	rules := []*Rule{
		{SourceEventName: "Event1", TargetMethod: "Method1"},
		{SourceEventName: "Event2", TargetMethod: "Method2"},
		{SourceEventName: "Event3", TargetMethod: "Method3"},
	}

	engine.RegisterRules(rules)

	all := engine.GetAllRules()
	if len(all) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(all))
	}
}

func TestEngine_MapFields_WithMapping(t *testing.T) {
	engine := NewEngine()
	engine.RegisterRule(&Rule{
		SourceEventName: "Event1",
		TargetMethod:    "Method1",
		RequiredFields:  []string{"id", "hash"},
		FieldMapping: map[string]string{
			"id":   "tokenId",
			"hash": "originHash",
		},
	})

	source := map[string]interface{}{
		"id":    "001",
		"hash":  "0xabc",
		"extra": "ignored",
	}

	result, err := engine.MapFields("Event1", source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["tokenId"] != "001" {
		t.Errorf("expected tokenId '001', got '%v'", result["tokenId"])
	}
	if result["originHash"] != "0xabc" {
		t.Errorf("expected originHash '0xabc', got '%v'", result["originHash"])
	}
	if _, ok := result["extra"]; ok {
		t.Error("extra field should not be in result")
	}
}

func TestEngine_MapFields_MissingRequiredField(t *testing.T) {
	engine := NewEngine()
	engine.RegisterRule(&Rule{
		SourceEventName: "Event1",
		RequiredFields:  []string{"id", "hash"},
	})

	source := map[string]interface{}{
		"id": "001",
		// "hash" is missing
	}

	_, err := engine.MapFields("Event1", source)
	if err == nil {
		t.Fatal("expected error for missing required field")
	}
}

func TestEngine_MapFields_NoMapping(t *testing.T) {
	engine := NewEngine()
	engine.RegisterRule(&Rule{
		SourceEventName: "Event1",
	})

	source := map[string]interface{}{
		"id":   "001",
		"hash": "0xabc",
	}

	result, err := engine.MapFields("Event1", source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 没有映射规则时返回原始字段
	if result["id"] != "001" {
		t.Errorf("expected id '001', got '%v'", result["id"])
	}
}

// --- 中间件测试 ---

func TestMiddlewareChain_Execute(t *testing.T) {
	chain := NewMiddlewareChain()

	// 注册两个中间件
	chain.Register(NewFuncMiddleware("addPrefix", func(ctx context.Context, fields map[string]interface{}) (map[string]interface{}, error) {
		fields["prefix"] = testPrefix
		return fields, nil
	}))
	chain.Register(NewFuncMiddleware("addSuffix", func(ctx context.Context, fields map[string]interface{}) (map[string]interface{}, error) {
		fields["suffix"] = testSuffix
		return fields, nil
	}))

	fields := map[string]interface{}{"data": "test"}
	result, err := chain.Execute(context.Background(), []string{"addPrefix", "addSuffix"}, fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["prefix"] != testPrefix {
		t.Errorf("expected prefix '%s', got '%v'", testPrefix, result["prefix"])
	}
	if result["suffix"] != testSuffix {
		t.Errorf("expected suffix '%s', got '%v'", testSuffix, result["suffix"])
	}
}

func TestMiddlewareChain_Execute_NotFound(t *testing.T) {
	chain := NewMiddlewareChain()

	fields := map[string]interface{}{}
	_, err := chain.Execute(context.Background(), []string{"nonexistent"}, fields)
	if err == nil {
		t.Fatal("expected error for non-existent middleware")
	}
}

func TestMiddlewareChain_Execute_Error(t *testing.T) {
	chain := NewMiddlewareChain()
	chain.Register(NewFuncMiddleware("failing", func(ctx context.Context, fields map[string]interface{}) (map[string]interface{}, error) {
		return nil, errors.New("middleware error")
	}))

	fields := map[string]interface{}{}
	_, err := chain.Execute(context.Background(), []string{"failing"}, fields)
	if err == nil {
		t.Fatal("expected error from failing middleware")
	}
}

func TestMiddlewareChain_ExecuteOrder(t *testing.T) {
	chain := NewMiddlewareChain()

	var order []string
	chain.Register(NewFuncMiddleware("first", func(ctx context.Context, fields map[string]interface{}) (map[string]interface{}, error) {
		order = append(order, "first")
		return fields, nil
	}))
	chain.Register(NewFuncMiddleware("second", func(ctx context.Context, fields map[string]interface{}) (map[string]interface{}, error) {
		order = append(order, "second")
		return fields, nil
	}))

	fields := map[string]interface{}{}
	_, _ = chain.Execute(context.Background(), []string{"first", "second"}, fields)

	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("expected order [first, second], got %v", order)
	}
}

func TestFuncMiddleware_Name(t *testing.T) {
	mw := NewFuncMiddleware("test-mw", nil)
	if mw.Name() != "test-mw" {
		t.Errorf("expected name 'test-mw', got '%s'", mw.Name())
	}
}
