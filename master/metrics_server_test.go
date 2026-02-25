package master

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestResolveMetricsAddrDefaultFromMasterPort(t *testing.T) {
	t.Setenv("MR_METRICS_ADDR", "")
	if got := resolveMetricsAddr(":10000"); got != "127.0.0.1:11000" {
		t.Fatalf("resolveMetricsAddr(:10000)=%s want 127.0.0.1:11000", got)
	}
	if got := resolveMetricsAddr("10.0.0.11:11340"); got != "10.0.0.11:12340" {
		t.Fatalf("resolveMetricsAddr host:11340=%s want 10.0.0.11:12340", got)
	}
}

func TestResolveMetricsAddrEnvOverride(t *testing.T) {
	t.Setenv("MR_METRICS_ADDR", ":2112")
	if got := resolveMetricsAddr(":10000"); got != ":2112" {
		t.Fatalf("resolveMetricsAddr env override=%s want :2112", got)
	}
}

func TestMetricsSnapshotIncludesCoreMetrics(t *testing.T) {
	_ = os.Setenv("MR_METRICS_ADDR", "")
	ms := NewMaster(1, 1).(*Master)
	ms.incTaskTotal("map", "submitted")
	ms.observeStageDuration("map", 2*time.Second)
	ms.metrics.IncTaskRetry("map")
	ms.metrics.SetWorkerAlive(1)

	out := ms.metricsSnapshot()
	for _, key := range []string{"task_total", "task_retry_total", "worker_alive", "stage_duration_seconds"} {
		if !strings.Contains(out, key) {
			t.Fatalf("metrics snapshot missing %q\n%s", key, out)
		}
	}
}
