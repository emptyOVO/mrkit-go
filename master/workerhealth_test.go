package master

import (
	"testing"
	"time"

	"github.com/emptyOVO/mrkit-go/rpc"
	"github.com/emptyOVO/mrkit-go/runtime/workerpool"
	"google.golang.org/grpc"
)

type healthClient struct {
	stateByIP map[string]int
}

func (h healthClient) Connect(workerIP string) (*grpc.ClientConn, rpc.WorkerClient) { return nil, nil }
func (h healthClient) Map(workerIP string, m *rpc.MapInfo) bool                     { return true }
func (h healthClient) Reduce(workerIP string, m *rpc.ReduceInfo) bool               { return true }
func (h healthClient) End(workerIP string) bool                                     { return true }
func (h healthClient) Health(workerIP string) int {
	if s, ok := h.stateByIP[workerIP]; ok {
		return s
	}
	return WORKER_UNKNOWN
}

func TestCheckWorkersHealthSyncsRegistryState(t *testing.T) {
	ms := NewMaster(2, 1).(*Master)
	ms.client = healthClient{stateByIP: map[string]int{
		"ip-idle": WORKER_IDLE,
		"ip-busy": WORKER_BUSY,
	}}
	ms.registry = workerpool.New(2 * time.Second)
	ms.Workers = append(ms.Workers,
		newWorker("w-idle", "ip-idle"),
		newWorker("w-busy", "ip-busy"),
	)
	ms.numWorkers = 2
	ms.registry.Register("w-idle", "ip-idle")
	ms.registry.Register("w-busy", "ip-busy")

	ms.checkWorkersHealth()

	stateByID := map[string]workerpool.WorkerState{}
	for _, w := range ms.registry.Snapshot() {
		stateByID[w.ID] = w.State
	}
	if stateByID["w-idle"] != workerpool.WorkerIdle {
		t.Fatalf("expected w-idle state idle in registry, got %q", stateByID["w-idle"])
	}
	if stateByID["w-busy"] != workerpool.WorkerBusy {
		t.Fatalf("expected w-busy state busy in registry, got %q", stateByID["w-busy"])
	}
	if !ms.Workers[0].Health() {
		t.Fatalf("expected worker w-idle to be idle")
	}
	if ms.Workers[1].Health() {
		t.Fatalf("expected worker w-busy to be non-idle")
	}
}

func TestCheckWorkersHealthEvictsExpiredWorker(t *testing.T) {
	ms := NewMaster(1, 1).(*Master)
	ms.client = healthClient{stateByIP: map[string]int{
		"ip-dead": WORKER_UNKNOWN,
	}}
	ms.registry = workerpool.New(10 * time.Millisecond)
	ms.Workers = append(ms.Workers, newWorker("w-dead", "ip-dead"))
	ms.numWorkers = 1
	ms.registry.Register("w-dead", "ip-dead")

	time.Sleep(15 * time.Millisecond)
	ms.checkWorkersHealth()

	if got := len(ms.registry.Snapshot()); got != 0 {
		t.Fatalf("expected expired worker evicted from registry, got %d", got)
	}
	if !ms.Workers[0].Broken() {
		t.Fatalf("expected worker w-dead marked as unknown")
	}
}
