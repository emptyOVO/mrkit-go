package observability

import (
	"strings"
	"testing"
	"time"
)

func TestRenderPrometheusContainsRequiredMetrics(t *testing.T) {
	m := NewMetrics()
	m.IncTaskTotal("map", "submitted")
	m.IncTaskTotal("map", "success")
	m.IncTaskRetry("map")
	m.SetWorkerAlive(3)
	m.ObserveStageDuration("map", 1500*time.Millisecond)

	text := m.RenderPrometheus()
	checks := []string{
		"task_total{stage=\"map\",result=\"submitted\"} 1",
		"task_total{stage=\"map\",result=\"success\"} 1",
		"task_retry_total{stage=\"map\"} 1",
		"worker_alive 3",
		"stage_duration_seconds{stage=\"map\"} 1.500000",
	}
	for _, c := range checks {
		if !strings.Contains(text, c) {
			t.Fatalf("render output missing %q\n%s", c, text)
		}
	}
}
