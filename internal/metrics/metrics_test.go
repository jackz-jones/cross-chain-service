package metrics

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- Metrics 测试 ---

func TestCrossChainMetrics_RecordEventReceived(t *testing.T) {
	collector := NewInMemoryCollector()
	m := NewCrossChainMetrics(collector)

	m.RecordEventReceived("EnterpriseNotifiedEvent", "chain1")
	m.RecordEventReceived("EnterpriseNotifiedEvent", "chain1")

	val := collector.GetValue("cross_chain_event_received_total", Labels{"event": "EnterpriseNotifiedEvent", "chain": "chain1"})
	if val != 2 {
		t.Errorf("expected 2, got %f", val)
	}
}

func TestCrossChainMetrics_RecordEventProcessed(t *testing.T) {
	collector := NewInMemoryCollector()
	m := NewCrossChainMetrics(collector)

	m.RecordEventProcessed("FileNotifiedEvent", "chain2")

	val := collector.GetValue("cross_chain_event_processed_total", Labels{"event": "FileNotifiedEvent", "chain": "chain2"})
	if val != 1 {
		t.Errorf("expected 1, got %f", val)
	}
}

func TestCrossChainMetrics_RecordEventFailed(t *testing.T) {
	collector := NewInMemoryCollector()
	m := NewCrossChainMetrics(collector)

	m.RecordEventFailed("CrossChainTransferEvent", "chain1", "timeout")

	val := collector.GetValue("cross_chain_event_failed_total", Labels{"event": "CrossChainTransferEvent", "chain": "chain1", "reason": "timeout"})
	if val != 1 {
		t.Errorf("expected 1, got %f", val)
	}
}

func TestCrossChainMetrics_RecordCrossChainTx(t *testing.T) {
	collector := NewInMemoryCollector()
	m := NewCrossChainMetrics(collector)

	m.RecordCrossChainTx("chain1", "chain2", "CrossChainMint", 2*time.Second)

	txVal := collector.GetValue("cross_chain_tx_submitted_total", Labels{"source": "chain1", "target": "chain2", "method": "CrossChainMint"})
	if txVal != 1 {
		t.Errorf("expected tx count 1, got %f", txVal)
	}

	durVal := collector.GetValue("cross_chain_tx_duration_seconds", Labels{"source": "chain1", "target": "chain2", "method": "CrossChainMint"})
	if durVal != 2.0 {
		t.Errorf("expected duration 2.0, got %f", durVal)
	}
}

func TestCrossChainMetrics_SetActiveChains(t *testing.T) {
	collector := NewInMemoryCollector()
	m := NewCrossChainMetrics(collector)

	m.SetActiveChains(3)

	val := collector.GetValue("cross_chain_active_chains", Labels{})
	if val != 3 {
		t.Errorf("expected 3, got %f", val)
	}
}

// --- 健康检查测试 ---

func TestHealthCheck_AllHealthy(t *testing.T) {
	hc := NewHealthCheck()
	hc.Register("redis", func() error { return nil })
	hc.Register("chain_service", func() error { return nil })

	resp := hc.Check()
	if resp.Status != HealthStatusUp {
		t.Errorf("expected UP, got %s", resp.Status)
	}
	if !hc.IsHealthy() {
		t.Error("expected IsHealthy to return true")
	}
}

func TestHealthCheck_OneDown(t *testing.T) {
	hc := NewHealthCheck()
	hc.Register("redis", func() error { return nil })
	hc.Register("chain_service", func() error { return errors.New("connection refused") })

	resp := hc.Check()
	if resp.Status != HealthStatusDown {
		t.Errorf("expected DOWN, got %s", resp.Status)
	}
	if hc.IsHealthy() {
		t.Error("expected IsHealthy to return false")
	}
	if resp.Checks["chain_service"] != "DOWN: connection refused" {
		t.Errorf("unexpected check result: %s", resp.Checks["chain_service"])
	}
}

func TestHealthCheck_HTTPHandler_Healthy(t *testing.T) {
	hc := NewHealthCheck()
	hc.Register("test", func() error { return nil })

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	hc.HTTPHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHealthCheck_HTTPHandler_Unhealthy(t *testing.T) {
	hc := NewHealthCheck()
	hc.Register("test", func() error { return errors.New("down") })

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	hc.HTTPHandler()(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHealthCheck_Empty(t *testing.T) {
	hc := NewHealthCheck()

	resp := hc.Check()
	if resp.Status != HealthStatusUp {
		t.Errorf("expected UP for empty checks, got %s", resp.Status)
	}
}
