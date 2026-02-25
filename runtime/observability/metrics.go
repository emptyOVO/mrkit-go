package observability

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Metrics stores lightweight Prometheus-compatible runtime metrics.
type Metrics struct {
	mu           sync.Mutex
	taskTotal    map[string]uint64
	taskRetry    map[string]uint64
	workerAlive  int
	stageLastDur map[string]float64
}

func NewMetrics() *Metrics {
	return &Metrics{
		taskTotal:    make(map[string]uint64),
		taskRetry:    make(map[string]uint64),
		stageLastDur: make(map[string]float64),
	}
}

func taskKey(stage, result string) string {
	return stage + "|" + result
}

func (m *Metrics) IncTaskTotal(stage, result string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.taskTotal[taskKey(stage, result)]++
}

func (m *Metrics) IncTaskRetry(stage string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.taskRetry[stage]++
}

func (m *Metrics) SetWorkerAlive(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n < 0 {
		n = 0
	}
	m.workerAlive = n
}

func (m *Metrics) ObserveStageDuration(stage string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stageLastDur[stage] = d.Seconds()
}

func escapeLabel(v string) string {
	v = strings.ReplaceAll(v, `\\`, `\\\\`)
	v = strings.ReplaceAll(v, `"`, `\\"`)
	v = strings.ReplaceAll(v, "\n", `\\n`)
	return v
}

func (m *Metrics) RenderPrometheus() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var b strings.Builder
	b.WriteString("# HELP task_total Total number of task lifecycle events by stage and result.\n")
	b.WriteString("# TYPE task_total counter\n")

	taskKeys := make([]string, 0, len(m.taskTotal))
	for k := range m.taskTotal {
		taskKeys = append(taskKeys, k)
	}
	sort.Strings(taskKeys)
	for _, k := range taskKeys {
		parts := strings.SplitN(k, "|", 2)
		stage := ""
		result := ""
		if len(parts) > 0 {
			stage = parts[0]
		}
		if len(parts) > 1 {
			result = parts[1]
		}
		fmt.Fprintf(&b, "task_total{stage=\"%s\",result=\"%s\"} %d\n", escapeLabel(stage), escapeLabel(result), m.taskTotal[k])
	}

	b.WriteString("# HELP task_retry_total Total number of task retries by stage.\n")
	b.WriteString("# TYPE task_retry_total counter\n")
	retryStages := make([]string, 0, len(m.taskRetry))
	for stage := range m.taskRetry {
		retryStages = append(retryStages, stage)
	}
	sort.Strings(retryStages)
	for _, stage := range retryStages {
		fmt.Fprintf(&b, "task_retry_total{stage=\"%s\"} %d\n", escapeLabel(stage), m.taskRetry[stage])
	}

	b.WriteString("# HELP worker_alive Number of alive workers in current registry lease window.\n")
	b.WriteString("# TYPE worker_alive gauge\n")
	fmt.Fprintf(&b, "worker_alive %d\n", m.workerAlive)

	b.WriteString("# HELP stage_duration_seconds Last observed stage duration in seconds.\n")
	b.WriteString("# TYPE stage_duration_seconds gauge\n")
	stages := make([]string, 0, len(m.stageLastDur))
	for stage := range m.stageLastDur {
		stages = append(stages, stage)
	}
	sort.Strings(stages)
	for _, stage := range stages {
		fmt.Fprintf(&b, "stage_duration_seconds{stage=\"%s\"} %.6f\n", escapeLabel(stage), m.stageLastDur[stage])
	}

	return b.String()
}
